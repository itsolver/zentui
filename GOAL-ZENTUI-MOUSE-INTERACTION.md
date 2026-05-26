# Goal: Zentui Mouse Interaction For Ticket Switching And Actions

## Objective

Add pragmatic mouse support to the `zentui tui` operator so Angus can switch tickets and trigger common actions by clicking, while preserving every existing keyboard workflow and confirmation guard.

The experience should stay terminal-native and cmux-like: click a ticket in the left queue to focus/load it, click panes to change focus, use the mouse wheel to scroll the pane under the pointer, and click visible action labels/buttons for draft, merge, open, refresh, timer, and approval flows.

## Constraints

- Keep keyboard shortcuts as the primary reliable path; mouse is additive.
- Do not introduce a full Textual/native GUI layer.
- Use Bubble Tea v2 mouse support in the existing TUI architecture.
- Public Zendesk updates, merges, requester cleanup, and status/time writes must still require explicit confirmation.
- Do not parse rendered strings to infer actions. Track clickable regions deliberately during layout/rendering.
- Prefer cell-motion mouse mode over all-motion unless a specific interaction requires full pointer movement.

## Relevant Code

- `cmd/tui.go`: TUI program startup.
- `internal/tui/app.go`: top-level state, layout, view rendering, and event routing.
- `internal/tui/list.go`: queue rows, cursor, pagination, search/filter state.
- `internal/tui/detail.go`: ticket detail timeline viewport.
- `internal/tui/operator.go`: draft, merge, timer, assets, AI/status panes.
- `internal/tui/actions.go`: comment/status/update approval modals.
- `internal/tui/cmdpalette.go`: command palette overlay.
- `internal/tui/*_test.go`: TUI state transition and rendering tests.

## To-Do

- [ ] Add a small hit-region model in `internal/tui` for clickable areas:
  - region id/action
  - x/y bounds in terminal cells
  - optional ticket id or action payload
  - disabled/danger metadata where useful for tests
- [ ] Enable mouse reporting in every `App.View()` return path by setting `tea.View.MouseMode = tea.MouseModeCellMotion`.
- [ ] Route `tea.MouseClickMsg`, `tea.MouseWheelMsg`, and relevant drag/release events in `App.Update()`.
- [ ] Record queue row hit regions while rendering the visible ticket list.
- [ ] Implement single-click ticket switching:
  - click visible ticket row
  - set list cursor to that row
  - focus the list pane
  - load/show the ticket detail in split/detail mode
  - start/pause/reset the per-ticket timer exactly as keyboard navigation does
- [ ] Implement pane focus and wheel behavior:
  - click left pane focuses queue
  - click main timeline focuses detail
  - click right operator pane focuses operator/action context where applicable
  - wheel over queue scrolls ticket selection
  - wheel over timeline scrolls the detail viewport
  - wheel over command palette/action modals scrolls or moves selection when supported
- [ ] Make the bottom command bar clickable for safe actions:
  - open in Zendesk
  - draft
  - merge preview
  - refresh
  - load more
  - images/assets
  - pause/reset timer
  - command palette
- [ ] Make modal/approval actions clickable without reducing safety:
  - confirm/cancel comment posting
  - choose public/internal where the modal supports it
  - choose/confirm status
  - choose merge suggestion
  - confirm/cancel merge
  - never submit a public update or merge from a single ambiguous click
- [ ] Add tests for:
  - hit-region bounds and priority when regions overlap
  - clicking a queue row selects and loads the correct ticket
  - clicking a command bar action calls the same path as the matching keybinding
  - mouse wheel events affect only the pane under the pointer
  - confirmation actions remain explicit for posting and merging
  - mouse mode is enabled on normal, overlay, and command-palette views
- [ ] Add manual smoke notes to the PR:
  - `go test ./...`
  - `go vet ./...`
  - `go build -o zentui`
  - `./zentui tui --demo`
  - live read-only queue load with `./zentui tui --view-id 7484423111055 --limit 20`

## Out Of Scope For V1

- Drag-and-drop ticket reordering.
- Hover-only tooltips or previews.
- Native GUI widgets.
- Changing Zendesk data without the existing confirmation flow.
- Replacing keyboard shortcuts or making mouse support mandatory.

## Done When

- Tickets can be changed by clicking rows in the queue.
- The visible common actions can be clicked.
- Mouse wheel scrolling works in the queue and ticket timeline.
- All existing keyboard shortcuts still work.
- Posting, merging, requester cleanup, status changes, and time writes still require explicit approval.
- Focused tests and `go test ./... && go vet ./...` pass.
