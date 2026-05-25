package triage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
