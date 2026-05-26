package triage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PromptPack struct {
	Status   string          `json:"status"`
	Kind     string          `json:"kind"`
	TicketID string          `json:"ticket_id"`
	Mode     string          `json:"mode"`
	Schema   json.RawMessage `json:"schema"`
	Prompt   string          `json:"prompt"`
	Error    string          `json:"error,omitempty"`
}

type DraftOutput struct {
	Answer            string `json:"answer"`
	RecommendedStatus string `json:"recommended_status"`
	ReasoningSummary  string `json:"reasoning_summary"`
}

type ImageOutput struct {
	Summary           string `json:"summary"`
	VisibleText       string `json:"visible_text"`
	IsSignatureOrLogo bool   `json:"is_signature_or_logo"`
	Relevance         string `json:"relevance"`
}

type MergePool struct {
	Status       string           `json:"status"`
	SourceTicket map[string]any   `json:"source_ticket"`
	Candidates   []map[string]any `json:"candidates"`
	Keywords     []string         `json:"keywords,omitempty"`
	Error        string           `json:"error,omitempty"`
}

type MergeSuggestion struct {
	ID             int64  `json:"id"`
	Subject        string `json:"subject"`
	Description    string `json:"description,omitempty"`
	Status         string `json:"status"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	RequesterID    int64  `json:"requester_id,omitempty"`
	OrganizationID int64  `json:"organization_id,omitempty"`
	RelevanceScore int    `json:"relevance_score"`
	Rationale      string `json:"rationale"`
}

type MergeNormalizeResult struct {
	Suggestions         []MergeSuggestion `json:"suggestions"`
	RecommendedTargetID int64             `json:"recommended_target_id,omitempty"`
}

func BuildDraftPromptPack(ctx context.Context, customerSupportDir string, pythonBin string, ticketID int64, mode string, imageObservations any, helperEnv ...string) (*PromptPack, error) {
	customerSupportDir, pythonBin, err := resolvePromptPackRuntime(customerSupportDir, pythonBin)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"mode": mode}
	if imageObservations != nil {
		payload["image_observations"] = imageObservations
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, pythonBin, "scripts/local_triage_codex.py", "draft-pack", fmt.Sprint(ticketID))
	cmd.Dir = customerSupportDir
	cmd.Stdin = bytes.NewReader(body)

	out, err := runPromptPackHelper(cmd, "building draft prompt pack", helperEnv)
	if err != nil {
		return nil, err
	}

	var pack PromptPack
	if err := json.Unmarshal(out, &pack); err != nil {
		return nil, err
	}
	if pack.Status != "success" {
		if pack.Error == "" {
			pack.Error = "prompt pack helper did not return success"
		}
		return &pack, errors.New(pack.Error)
	}
	return &pack, nil
}

func BuildImagePromptPack(ctx context.Context, customerSupportDir string, pythonBin string, ticketID int64, filename string, sourceURL string, commentContext string, helperEnv ...string) (*PromptPack, error) {
	customerSupportDir, pythonBin, err := resolvePromptPackRuntime(customerSupportDir, pythonBin)
	if err != nil {
		return nil, err
	}
	args := []string{"scripts/local_triage_codex.py", "image-pack", fmt.Sprint(ticketID), "--filename", filename}
	if sourceURL != "" {
		args = append(args, "--source-url", sourceURL)
	}
	if commentContext != "" {
		args = append(args, "--comment-context", commentContext)
	}
	cmd := exec.CommandContext(ctx, pythonBin, args...)
	cmd.Dir = customerSupportDir

	out, err := runPromptPackHelper(cmd, "building image prompt pack", helperEnv)
	if err != nil {
		return nil, err
	}
	var pack PromptPack
	if err := json.Unmarshal(out, &pack); err != nil {
		return nil, err
	}
	if pack.Status != "success" {
		if pack.Error == "" {
			pack.Error = "image prompt pack helper did not return success"
		}
		return &pack, errors.New(pack.Error)
	}
	return &pack, nil
}

func BuildMergePool(ctx context.Context, customerSupportDir string, pythonBin string, ticketID int64, helperEnv ...string) (*MergePool, error) {
	customerSupportDir, pythonBin, err := resolvePromptPackRuntime(customerSupportDir, pythonBin)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, pythonBin, "scripts/local_triage_codex.py", "merge-pool", fmt.Sprint(ticketID))
	cmd.Dir = customerSupportDir

	out, err := runPromptPackHelper(cmd, "building merge candidate pool", helperEnv)
	if err != nil {
		return nil, err
	}
	var pool MergePool
	if err := json.Unmarshal(out, &pool); err != nil {
		return nil, err
	}
	if pool.Status == "error" {
		if pool.Error == "" {
			pool.Error = "merge pool helper returned an error"
		}
		return &pool, errors.New(pool.Error)
	}
	return &pool, nil
}

func BuildMergePromptPack(ctx context.Context, customerSupportDir string, pythonBin string, sourceTicket map[string]any, candidates []map[string]any, helperEnv ...string) (*PromptPack, error) {
	customerSupportDir, pythonBin, err := resolvePromptPackRuntime(customerSupportDir, pythonBin)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"source_ticket": sourceTicket,
		"candidates":    candidates,
	})
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, pythonBin, "scripts/local_triage_codex.py", "merge-pack")
	cmd.Dir = customerSupportDir
	cmd.Stdin = bytes.NewReader(body)

	out, err := runPromptPackHelper(cmd, "building merge prompt pack", helperEnv)
	if err != nil {
		return nil, err
	}
	var pack PromptPack
	if err := json.Unmarshal(out, &pack); err != nil {
		return nil, err
	}
	if pack.Status != "success" {
		if pack.Error == "" {
			pack.Error = "merge prompt pack helper did not return success"
		}
		return &pack, errors.New(pack.Error)
	}
	return &pack, nil
}

func NormalizeMergePromptPackResult(ctx context.Context, customerSupportDir string, pythonBin string, codexPayload json.RawMessage, candidates []map[string]any, helperEnv ...string) (MergeNormalizeResult, error) {
	customerSupportDir, pythonBin, err := resolvePromptPackRuntime(customerSupportDir, pythonBin)
	if err != nil {
		return MergeNormalizeResult{}, err
	}
	var payload any
	if err := json.Unmarshal(codexPayload, &payload); err != nil {
		return MergeNormalizeResult{}, err
	}
	body, err := json.Marshal(map[string]any{
		"codex_payload": payload,
		"candidates":    candidates,
	})
	if err != nil {
		return MergeNormalizeResult{}, err
	}
	cmd := exec.CommandContext(ctx, pythonBin, "scripts/local_triage_codex.py", "normalize-merge")
	cmd.Dir = customerSupportDir
	cmd.Stdin = bytes.NewReader(body)

	out, err := runPromptPackHelper(cmd, "normalizing merge ranking", helperEnv)
	if err != nil {
		return MergeNormalizeResult{}, err
	}
	var normalized MergeNormalizeResult
	if err := json.Unmarshal(out, &normalized); err != nil {
		return MergeNormalizeResult{}, err
	}
	return normalized, nil
}

func NormalizeDraftPromptPackResult(ctx context.Context, customerSupportDir string, pythonBin string, mode string, output DraftOutput, helperEnv ...string) (DraftOutput, error) {
	customerSupportDir, pythonBin, err := resolvePromptPackRuntime(customerSupportDir, pythonBin)
	if err != nil {
		return DraftOutput{}, err
	}
	body, err := json.Marshal(output)
	if err != nil {
		return DraftOutput{}, err
	}

	cmd := exec.CommandContext(ctx, pythonBin, "scripts/local_triage_codex.py", "normalize-draft", "--mode", mode)
	cmd.Dir = customerSupportDir
	cmd.Stdin = bytes.NewReader(body)

	out, err := runPromptPackHelper(cmd, "normalizing draft", helperEnv)
	if err != nil {
		return DraftOutput{}, err
	}

	var normalized DraftOutput
	if err := json.Unmarshal(out, &normalized); err != nil {
		return DraftOutput{}, err
	}
	return normalized, nil
}

func resolvePromptPackRuntime(customerSupportDir string, pythonBin string) (string, string, error) {
	customerSupportDir = strings.TrimSpace(customerSupportDir)
	if customerSupportDir == "" {
		return "", "", errors.New("customer-support directory is required; set --customer-support-dir or ZENTUI_CUSTOMER_SUPPORT_DIR")
	}
	if pythonBin == "" {
		pythonBin = filepath.Join(customerSupportDir, ".venv", "bin", "python")
	}
	return customerSupportDir, pythonBin, nil
}

func runPromptPackHelper(cmd *exec.Cmd, action string, helperEnv []string) ([]byte, error) {
	if len(helperEnv) > 0 {
		cmd.Env = append(os.Environ(), helperEnv...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail != "" {
			return nil, fmt.Errorf("%s: %w: %s", action, err, truncateHelperOutput(detail))
		}
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	return stdout.Bytes(), nil
}

func truncateHelperOutput(value string) string {
	const maxLen = 1200
	if len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen]) + "..."
}
