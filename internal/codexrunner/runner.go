package codexrunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Runner struct {
	Binary             string
	CustomerSupportDir string
	Model              string
	ReasoningEffort    string
	Timeout            time.Duration
}

type Usage struct {
	InputTokens           int `json:"input_tokens,omitempty"`
	CachedInputTokens     int `json:"cached_input_tokens,omitempty"`
	OutputTokens          int `json:"output_tokens,omitempty"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens,omitempty"`
}

type Result struct {
	Output json.RawMessage `json:"output"`
	Stdout string          `json:"stdout"`
	Stderr string          `json:"stderr"`
	Usage  *Usage          `json:"usage,omitempty"`
}

func (r Runner) RunPrompt(ctx context.Context, prompt string, schema json.RawMessage, images []string) (*Result, error) {
	binary := r.Binary
	if binary == "" {
		binary = "codex"
	}
	if r.CustomerSupportDir == "" {
		return nil, fmt.Errorf("customer support directory is required")
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "zentui-codex-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	schemaPath := filepath.Join(tmpDir, "schema.json")
	if err := os.WriteFile(schemaPath, schema, 0o600); err != nil {
		return nil, fmt.Errorf("writing schema: %w", err)
	}
	outputPath := filepath.Join(tmpDir, "last-message.json")

	args := []string{
		"exec",
		"--json",
		"--ephemeral",
		"--sandbox",
		"read-only",
		"--cd",
		r.CustomerSupportDir,
		"--output-schema",
		schemaPath,
		"--output-last-message",
		outputPath,
		"--color",
		"never",
	}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	if r.ReasoningEffort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", r.ReasoningEffort))
	}
	for _, image := range images {
		if strings.TrimSpace(image) != "" {
			args = append(args, "--image", image)
		}
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = r.CustomerSupportDir
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &Result{Stdout: stdout.String(), Stderr: stderr.String(), Usage: ParseUsage(stdout.String())}, fmt.Errorf("running codex exec: %w", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("reading last message: %w", err)
	}
	output = bytes.TrimSpace(output)
	if !json.Valid(output) {
		return nil, fmt.Errorf("last message is not valid JSON")
	}

	return &Result{
		Output: append(json.RawMessage(nil), output...),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Usage:  ParseUsage(stdout.String()),
	}, nil
}

func ParseUsage(jsonl string) *Usage {
	var last *Usage
	scanner := bufio.NewScanner(strings.NewReader(jsonl))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event struct {
			Type  string `json:"type"`
			Usage *Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "turn.completed" && event.Usage != nil {
			copy := *event.Usage
			last = &copy
		}
	}
	return last
}

func DecodeOutput[T any](raw json.RawMessage) (T, error) {
	var out T
	if len(bytes.TrimSpace(raw)) == 0 {
		return out, fmt.Errorf("empty codex output")
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}
