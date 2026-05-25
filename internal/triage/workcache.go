package triage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/itsolver/zentui/internal/types"
	"github.com/itsolver/zentui/pkg/zendesk"
)

const (
	DefaultWorkDir = ".ticket-triage-work"
	MaxImageBytes  = int64(10 << 20)
)

var inlineURLRe = regexp.MustCompile(`https?://[^\s"'<>]+`)

type WorkCache struct {
	Root       string
	HTTPClient *http.Client
}

type ImageSource struct {
	URL         string `json:"url"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Inline      bool   `json:"inline,omitempty"`
	CommentID   int64  `json:"comment_id,omitempty"`
}

type AssetRecord struct {
	SourceURL    string    `json:"source_url"`
	SHA256       string    `json:"sha256,omitempty"`
	Filename     string    `json:"filename"`
	LocalPath    string    `json:"local_path,omitempty"`
	ContentType  string    `json:"content_type,omitempty"`
	Size         int64     `json:"size,omitempty"`
	DownloadedAt time.Time `json:"downloaded_at,omitempty"`
	Skipped      bool      `json:"skipped,omitempty"`
	SkipReason   string    `json:"skip_reason,omitempty"`
}

type Manifest struct {
	TicketID  int64         `json:"ticket_id"`
	Assets    []AssetRecord `json:"assets"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type ImageAnalysis struct {
	Summary           string `json:"summary"`
	VisibleText       string `json:"visible_text"`
	IsSignatureOrLogo bool   `json:"is_signature_or_logo"`
	Relevance         string `json:"relevance"`
}

func (c WorkCache) ticketDir(ticketID int64) string {
	root := c.Root
	if root == "" {
		root = DefaultWorkDir
	}
	if !filepath.IsAbs(root) {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}
	return filepath.Join(root, strconv.FormatInt(ticketID, 10))
}

func (c WorkCache) EnsureTicketDir(ticketID int64) (string, error) {
	dir := c.ticketDir(ticketID)
	if err := os.MkdirAll(filepath.Join(dir, "attachments"), 0o700); err != nil {
		return "", err
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		manifest := Manifest{TicketID: ticketID, UpdatedAt: time.Now().UTC()}
		if err := c.WriteManifest(ticketID, manifest); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func (c WorkCache) ReadManifest(ticketID int64) (Manifest, error) {
	var manifest Manifest
	path := filepath.Join(c.ticketDir(ticketID), "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{TicketID: ticketID}, nil
		}
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.TicketID == 0 {
		manifest.TicketID = ticketID
	}
	return manifest, nil
}

func (c WorkCache) WriteManifest(ticketID int64, manifest Manifest) error {
	dir := c.ticketDir(ticketID)
	if err := os.MkdirAll(filepath.Join(dir, "attachments"), 0o700); err != nil {
		return err
	}
	manifest.TicketID = ticketID
	manifest.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(data, '\n'), 0o600)
}

func ExtractImageSources(comments []types.Comment) []ImageSource {
	seen := map[string]bool{}
	var out []ImageSource
	add := func(source ImageSource) {
		if source.URL == "" || seen[source.URL] {
			return
		}
		if source.ContentType != "" && !strings.HasPrefix(source.ContentType, "image/") {
			return
		}
		if source.ContentType == "" && !looksLikeImageURL(source.URL) {
			return
		}
		seen[source.URL] = true
		out = append(out, source)
	}

	for _, comment := range comments {
		for _, attachment := range comment.Attachments {
			add(ImageSource{
				URL:         attachment.ContentURL,
				Filename:    attachment.FileName,
				ContentType: attachment.ContentType,
				Size:        attachment.Size,
				Inline:      attachment.Inline,
				CommentID:   comment.ID,
			})
		}
		for _, body := range []string{comment.Body, comment.HTMLBody, comment.PlainBody} {
			for _, raw := range inlineURLRe.FindAllString(body, -1) {
				cleaned := strings.TrimRight(raw, ".,);]")
				add(ImageSource{URL: cleaned, Filename: filenameFromURL(cleaned), Inline: true, CommentID: comment.ID})
			}
		}
	}
	return out
}

func ExtractImageSourcesFromAudits(audits []types.Audit) []ImageSource {
	var comments []types.Comment
	for _, audit := range audits {
		for _, event := range audit.Events {
			if event.Type != "Comment" {
				continue
			}
			comments = append(comments, types.Comment{
				ID:          event.ID,
				Body:        event.Body,
				HTMLBody:    event.HTMLBody,
				Public:      event.Public,
				AuthorID:    audit.AuthorID,
				Attachments: event.Attachments,
				CreatedAt:   audit.CreatedAt,
			})
		}
	}
	return ExtractImageSources(comments)
}

func (c WorkCache) DownloadImage(ctx context.Context, ticketID int64, source ImageSource) (AssetRecord, error) {
	if _, err := c.EnsureTicketDir(ticketID); err != nil {
		return AssetRecord{}, err
	}
	manifest, err := c.ReadManifest(ticketID)
	if err != nil {
		return AssetRecord{}, err
	}
	for _, asset := range manifest.Assets {
		if asset.SourceURL == source.URL {
			return asset, nil
		}
	}

	if source.Size > MaxImageBytes {
		asset := skippedAsset(source, "image is too large")
		return asset, c.addAsset(ticketID, asset)
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return AssetRecord{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return AssetRecord{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return AssetRecord{}, fmt.Errorf("downloading image: status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = source.ContentType
	}
	if !strings.HasPrefix(contentType, "image/") {
		asset := skippedAsset(source, "unsupported content type")
		asset.ContentType = contentType
		return asset, c.addAsset(ticketID, asset)
	}

	limited := io.LimitReader(resp.Body, MaxImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return AssetRecord{}, err
	}
	if int64(len(data)) > MaxImageBytes {
		asset := skippedAsset(source, "image is too large")
		asset.ContentType = contentType
		return asset, c.addAsset(ticketID, asset)
	}

	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	for _, asset := range manifest.Assets {
		if asset.SHA256 == sha && asset.SHA256 != "" && !asset.Skipped {
			return asset, nil
		}
	}

	filename := safeFilename(source.Filename)
	if filename == "" {
		filename = sha[:12] + extensionForContentType(contentType)
	}
	localPath := filepath.Join(c.ticketDir(ticketID), "attachments", filename)
	if _, err := os.Stat(localPath); err == nil {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		filename = base + "-" + sha[:8] + ext
		localPath = filepath.Join(c.ticketDir(ticketID), "attachments", filename)
	}
	if err := os.WriteFile(localPath, data, 0o600); err != nil {
		return AssetRecord{}, err
	}

	asset := AssetRecord{
		SourceURL:    source.URL,
		SHA256:       sha,
		Filename:     filename,
		LocalPath:    localPath,
		ContentType:  contentType,
		Size:         int64(len(data)),
		DownloadedAt: time.Now().UTC(),
	}
	return asset, c.addAsset(ticketID, asset)
}

func (c WorkCache) ReadImageAnalysis(ticketID int64) (map[string]ImageAnalysis, error) {
	path := filepath.Join(c.ticketDir(ticketID), "image-analysis.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ImageAnalysis{}, nil
		}
		return nil, err
	}
	var out map[string]ImageAnalysis
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]ImageAnalysis{}
	}
	return out, nil
}

func (c WorkCache) WriteImageAnalysis(ticketID int64, analysis map[string]ImageAnalysis) error {
	if _, err := c.EnsureTicketDir(ticketID); err != nil {
		return err
	}
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.ticketDir(ticketID), "image-analysis.json"), append(data, '\n'), 0o600)
}

func (c WorkCache) AppendCodexRun(ticketID int64, record any) error {
	if _, err := c.EnsureTicketDir(ticketID); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	path := filepath.Join(c.ticketDir(ticketID), "codex-runs.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func CleanupClosed(ctx context.Context, root string, tickets zendesk.TicketService) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil {
			continue
		}
		result, err := tickets.Get(ctx, id, nil)
		if err != nil || result.Ticket.Status != "closed" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (c WorkCache) addAsset(ticketID int64, asset AssetRecord) error {
	manifest, err := c.ReadManifest(ticketID)
	if err != nil {
		return err
	}
	manifest.Assets = append(manifest.Assets, asset)
	return c.WriteManifest(ticketID, manifest)
}

func skippedAsset(source ImageSource, reason string) AssetRecord {
	filename := safeFilename(source.Filename)
	if filename == "" {
		filename = filenameFromURL(source.URL)
	}
	return AssetRecord{
		SourceURL:   source.URL,
		Filename:    filename,
		ContentType: source.ContentType,
		Size:        source.Size,
		Skipped:     true,
		SkipReason:  reason,
	}
}

func looksLikeImageURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	}
	return false
}

func filenameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return safeFilename(filepath.Base(parsed.Path))
}

func safeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "\x00", "")
	name = replacer.Replace(name)
	name = strings.Trim(name, ". ")
	if len(name) > 120 {
		ext := filepath.Ext(name)
		if len(ext) >= 120 {
			name = name[:120]
		} else {
			base := strings.TrimSuffix(name, ext)
			name = base[:120-len(ext)] + ext
		}
	}
	return name
}

func extensionForContentType(contentType string) string {
	switch strings.Split(contentType, ";")[0] {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".img"
	}
}
