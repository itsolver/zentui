package triage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractImageSourcesDedupeAndSkipsUnsupportedAttachments(t *testing.T) {
	pub := true
	comments := []types.Comment{{
		ID:     1,
		Body:   `Please see https://uploads.example.test/screen.png and https://uploads.example.test/file.txt`,
		Public: &pub,
		Attachments: []types.Attachment{
			{FileName: "screen.png", ContentURL: "https://uploads.example.test/screen.png", ContentType: "image/png", Size: 100},
			{FileName: "notes.txt", ContentURL: "https://uploads.example.test/notes.txt", ContentType: "text/plain", Size: 100},
		},
	}}

	sources := ExtractImageSources(comments)

	require.Len(t, sources, 1)
	assert.Equal(t, "https://uploads.example.test/screen.png", sources[0].URL)
	assert.Equal(t, "screen.png", sources[0].Filename)
}

func TestExtractImageSourcesFromAudits(t *testing.T) {
	audits := []types.Audit{{
		ID:       10,
		AuthorID: 1,
		Events: []types.AuditEvent{{
			ID:       100,
			Type:     "Comment",
			HTMLBody: `<img src="https://uploads.example.test/a.png">`,
			Attachments: []types.Attachment{{
				FileName:    "b.jpg",
				ContentURL:  "https://uploads.example.test/b.jpg",
				ContentType: "image/jpeg",
			}},
		}},
	}}

	sources := ExtractImageSourcesFromAudits(audits)

	require.Len(t, sources, 2)
	assert.Equal(t, int64(100), sources[0].CommentID)
}

func TestDownloadImageWritesManifestAndReusesURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png-data"))
	}))
	defer server.Close()

	cache := WorkCache{Root: t.TempDir(), HTTPClient: server.Client()}
	source := ImageSource{URL: server.URL + "/screen.png", Filename: "../screen.png", ContentType: "image/png"}

	first, err := cache.DownloadImage(context.Background(), 123, source)
	require.NoError(t, err)
	second, err := cache.DownloadImage(context.Background(), 123, source)
	require.NoError(t, err)

	assert.Equal(t, first.LocalPath, second.LocalPath)
	assert.FileExists(t, first.LocalPath)
	assert.True(t, filepath.IsAbs(first.LocalPath))

	manifest, err := cache.ReadManifest(123)
	require.NoError(t, err)
	require.Len(t, manifest.Assets, 1)
	assert.Equal(t, "screen.png", manifest.Assets[0].Filename)
}

func TestDownloadImageSkipsUnsupportedContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("pdf"))
	}))
	defer server.Close()

	cache := WorkCache{Root: t.TempDir(), HTTPClient: server.Client()}
	asset, err := cache.DownloadImage(context.Background(), 123, ImageSource{URL: server.URL + "/file.pdf", Filename: "file.pdf"})

	require.NoError(t, err)
	assert.True(t, asset.Skipped)
	assert.Equal(t, "unsupported content type", asset.SkipReason)
}

func TestDownloadImageReusesSkippedURL(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("pdf"))
	}))
	defer server.Close()

	cache := WorkCache{Root: t.TempDir(), HTTPClient: server.Client()}
	source := ImageSource{URL: server.URL + "/file.pdf", Filename: "file.pdf"}

	first, err := cache.DownloadImage(context.Background(), 123, source)
	require.NoError(t, err)
	second, err := cache.DownloadImage(context.Background(), 123, source)
	require.NoError(t, err)

	assert.True(t, first.Skipped)
	assert.Equal(t, first, second)
	assert.Equal(t, int32(1), requests.Load())

	manifest, err := cache.ReadManifest(123)
	require.NoError(t, err)
	require.Len(t, manifest.Assets, 1)
}

func TestDownloadImageUsesAuthClientOnlyForTrustedHosts(t *testing.T) {
	trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png-data"))
	}))
	defer trusted.Close()
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png-data-2"))
	}))
	defer external.Close()

	trustedURL := mustParseURL(t, trusted.URL)
	cache := WorkCache{
		Root:         t.TempDir(),
		HTTPClient:   &http.Client{Transport: authHeaderTransport{base: http.DefaultTransport}},
		TrustedHosts: []string{trustedURL.Host},
	}

	_, err := cache.DownloadImage(context.Background(), 123, ImageSource{URL: trusted.URL + "/trusted.png", Filename: "trusted.png"})
	require.NoError(t, err)
	_, err = cache.DownloadImage(context.Background(), 123, ImageSource{URL: external.URL + "/external.png", Filename: "external.png"})
	require.NoError(t, err)
}

func TestWorkCacheTrustsZendeskContentHostSuffixes(t *testing.T) {
	cache := WorkCache{
		HTTPClient:   &http.Client{Transport: authHeaderTransport{base: http.DefaultTransport}},
		TrustedHosts: []string{".zdusercontent.com"},
	}

	assert.Same(t, cache.HTTPClient, cache.clientForSource("https://attachments.zdusercontent.com/asset.png"))
	assert.Same(t, http.DefaultClient, cache.clientForSource("https://notzdusercontent.com/asset.png"))
}

func TestSafeFilenameHandlesOversizedExtension(t *testing.T) {
	name := safeFilename("screen." + strings.Repeat("x", 200))

	assert.NotEmpty(t, name)
	assert.LessOrEqual(t, len(name), 120)
}

type authHeaderTransport struct {
	base http.RoundTripper
}

func (t authHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer secret")
	return t.base.RoundTrip(clone)
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	return parsed
}

func TestImageAnalysisReadWrite(t *testing.T) {
	cache := WorkCache{Root: t.TempDir()}
	analysis := map[string]ImageAnalysis{
		"sha": {Summary: "error dialog", VisibleText: "Error", IsSignatureOrLogo: false, Relevance: "high"},
	}

	require.NoError(t, cache.WriteImageAnalysis(123, analysis))
	got, err := cache.ReadImageAnalysis(123)

	require.NoError(t, err)
	assert.Equal(t, "error dialog", got["sha"].Summary)
}

func TestAppendCodexRun(t *testing.T) {
	cache := WorkCache{Root: t.TempDir()}

	require.NoError(t, cache.AppendCodexRun(123, map[string]any{"kind": "draft"}))
	require.NoError(t, cache.AppendCodexRun(123, map[string]any{"kind": "image"}))

	data, err := os.ReadFile(filepath.Join(cache.ticketDir(123), "codex-runs.jsonl"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"kind":"draft"`)
	assert.Contains(t, string(data), `"kind":"image"`)
}

func TestCleanupClosedOnlyRemovesClosedFolders(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "1"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "2"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "3"), 0o700))

	svc := cleanupTicketService{statuses: map[int64]string{
		1: "open",
		2: "solved",
		3: "closed",
	}}

	require.NoError(t, CleanupClosed(context.Background(), root, svc))

	assert.DirExists(t, filepath.Join(root, "1"))
	assert.DirExists(t, filepath.Join(root, "2"))
	assert.NoDirExists(t, filepath.Join(root, "3"))
}

type cleanupTicketService struct {
	statuses map[int64]string
}

func (s cleanupTicketService) List(context.Context, *types.ListTicketsOptions) (*types.TicketPage, error) {
	return nil, nil
}

func (s cleanupTicketService) ListView(context.Context, int64, *types.ListTicketsOptions) (*types.TicketPage, error) {
	return nil, nil
}

func (s cleanupTicketService) Get(_ context.Context, id int64, _ *types.GetTicketOptions) (*types.TicketResult, error) {
	status := s.statuses[id]
	if status == "" {
		status = "open"
	}
	return &types.TicketResult{Ticket: types.Ticket{ID: id, Status: status, UpdatedAt: time.Now()}}, nil
}

func (s cleanupTicketService) Create(context.Context, *types.CreateTicketRequest) (*types.Ticket, error) {
	return nil, nil
}

func (s cleanupTicketService) Update(context.Context, int64, *types.UpdateTicketRequest) (*types.Ticket, error) {
	return nil, nil
}

func (s cleanupTicketService) Delete(context.Context, int64) error {
	return nil
}

func (s cleanupTicketService) ListComments(context.Context, int64, *types.ListCommentsOptions) (*types.CommentPage, error) {
	return nil, nil
}

func (s cleanupTicketService) ListAudits(context.Context, int64, *types.ListAuditsOptions) (*types.AuditPage, error) {
	return nil, nil
}

func (s cleanupTicketService) ListTicketFields(context.Context, *types.ListTicketFieldsOptions) (*types.TicketFieldPage, error) {
	return nil, nil
}

func (s cleanupTicketService) MergeTickets(context.Context, int64, *types.MergeTicketsRequest) (*types.MergeTicketsResult, error) {
	return nil, nil
}
