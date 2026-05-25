package triage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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

func BuildDraftPromptPack(ctx context.Context, customerSupportDir string, pythonBin string, ticketID int64, mode string, imageObservations any) (*PromptPack, error) {
	if pythonBin == "" {
		pythonBin = filepath.Join(customerSupportDir, ".venv", "bin", "python")
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

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("building draft prompt pack: %w", err)
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

func NormalizeDraftPromptPackResult(ctx context.Context, customerSupportDir string, pythonBin string, mode string, output DraftOutput) (DraftOutput, error) {
	if pythonBin == "" {
		pythonBin = filepath.Join(customerSupportDir, ".venv", "bin", "python")
	}
	body, err := json.Marshal(output)
	if err != nil {
		return DraftOutput{}, err
	}

	cmd := exec.CommandContext(ctx, pythonBin, "scripts/local_triage_codex.py", "normalize-draft", "--mode", mode)
	cmd.Dir = customerSupportDir
	cmd.Stdin = bytes.NewReader(body)

	out, err := cmd.Output()
	if err != nil {
		return DraftOutput{}, fmt.Errorf("normalizing draft: %w", err)
	}

	var normalized DraftOutput
	if err := json.Unmarshal(out, &normalized); err != nil {
		return DraftOutput{}, err
	}
	return normalized, nil
}
