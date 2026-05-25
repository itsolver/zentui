# Repository Guidelines

## Project Structure & Module Organization

This is a Go CLI/TUI for Zendesk, module `github.com/itsolver/zentui`.

- `main.go` starts the Cobra command tree.
- `cmd/` contains CLI commands, MCP tool wiring, and command-level tests.
- `internal/api/`, `internal/auth/`, `internal/config/`, `internal/output/`, `internal/tui/`, `internal/demo/`, and `internal/nlq/` contain implementation packages.
- `pkg/zendesk/` exposes public service interfaces used across commands and MCP tooling.
- `testdata/` stores JSON fixtures for tests.
- `skills/zentui/` contains agent-facing reference material.
- `zentui-list.png` and `zentui-kanban.png` are README/TUI assets.

## Build, Test, and Development Commands

- `go build -o zentui` builds the local CLI binary.
- `go build ./...` checks all packages compile.
- `go test ./...` runs the full test suite.
- `go test ./internal/api/ -run TestName` runs one focused test.
- `go test -race -failfast ./...` mirrors the main CI test mode.
- `go vet ./...` runs static checks.
- `gofmt -w .` formats Go code before committing.
- `./zentui tui --demo` or `./zentui tickets list --demo` exercises the app without Zendesk credentials.

CI also verifies `go mod tidy && git diff --exit-code go.mod go.sum`; keep dependency files tidy.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs via `gofmt`, exported identifiers with comments when needed, and short package names. Keep `cmd/` handlers thin: parse flags, call services, and format output. Put reusable behavior in `internal/` packages or the existing service interfaces instead of adding command-local abstractions.

## Testing Guidelines

Tests are colocated as `*_test.go`. Prefer table tests for command and package behavior. API tests should use `net/http/httptest` and fixtures from `testdata/`; do not call live Zendesk services. Add or update tests for behavior changes, especially auth, output formatting, pagination, role gating, MCP tools, and TUI state transitions.

## Commit & Pull Request Guidelines

Commit messages must follow Conventional Commits, enforced by `lefthook`:
`feat: add ticket export command`, `fix(auth): handle expired OAuth tokens`, or `docs: update README`.

Before opening a PR, run `gofmt -w .`, `go test ./...`, and `go vet ./...`. PRs should describe the change, note testing performed, link any related issue, and include screenshots or terminal output for visible CLI/TUI changes.

## Agent-Specific Instructions

Make surgical changes. Match existing patterns, avoid speculative refactors, and keep user-facing CLI contracts stable unless the task explicitly changes them.
