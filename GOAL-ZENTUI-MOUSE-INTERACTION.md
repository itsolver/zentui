# Goal: Zentui Mouse Interaction For Ticket Switching And Actions

## Objective

Add pragmatic mouse support to the `zentui tui` operator so Angus can switch tickets, edit ticket fields, inspect assets, and trigger common actions by clicking, while preserving every existing keyboard workflow and confirmation guard.

The experience should stay terminal-native and cmux-like: click a ticket in the left queue to focus/load it, click panes to change focus, use the mouse wheel to scroll the pane under the pointer, click assets to open the ticket work folder in Finder, and click visible action labels/buttons for field editing, draft, merge, open, refresh, timer, and approval flows.

## Constraints

- Keep keyboard shortcuts as the primary reliable path; mouse is additive.
- Do not introduce a full Textual/native GUI layer.
- Use Bubble Tea v2 mouse support in the existing TUI architecture.
- Public Zendesk updates, merges, requester cleanup, and status/time writes must still require explicit confirmation.
- Do not parse rendered strings to infer actions. Track clickable regions deliberately during layout/rendering.
- Prefer cell-motion mouse mode over all-motion unless a specific interaction requires full pointer movement.
- Field editing must use the existing Zendesk update path and must be explicit: the user edits a value in a text box and confirms before `custom_fields` are written.
- Clicking assets should open the local `.ticket-triage-work/<ticket_id>/` folder or an individual downloaded file in Finder without modifying Zendesk.

## Relevant Code

- `cmd/tui.go`: TUI program startup.
- `internal/tui/app.go`: top-level state, layout, view rendering, and event routing.
- `internal/tui/list.go`: queue rows, cursor, pagination, search/filter state.
- `internal/tui/detail.go`: ticket detail timeline viewport.
- `internal/tui/operator.go`: draft, merge, timer, assets, AI/status panes.
- `internal/tui/actions.go`: comment/status/update approval modals.
- `internal/tui/cmdpalette.go`: command palette overlay.
- `internal/types/ticket_field.go`: ticket field metadata and option labels.
- `internal/types/ticket.go`: `CustomField` and `UpdateTicketRequest.CustomFields`.
- `internal/browser/` or a new small opener helper: open local Finder paths and Zendesk/browser URLs.
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
- [ ] Add ticket-field editing from the operator pane:
  - show editable custom fields in the right sidebar with their labels and current values
  - hide noisy/non-actionable fields already identified elsewhere
  - clicking a field opens an edit modal
  - the field value is edited in a text box, even for v1
  - preserve the current value as the default text
  - allow clearing a field only after explicit confirmation
  - submit a narrow `UpdateTicketRequest{CustomFields: []types.CustomField{{ID, Value}}}` update
  - refresh/reload the ticket after successful field update
- [ ] Keep field editing type handling conservative:
  - text/integer/decimal values are edited as text and sent as the minimal compatible JSON value
  - checkbox/dropdown/multiselect support can be read-only or text-slug based in v1 unless metadata makes a safe UI obvious
  - unsupported field types should display as read-only rather than guessing
  - validation errors from Zendesk should stay visible in the modal without losing the typed value
- [ ] Add clickable assets behavior:
  - clicking the Assets header or folder path opens `.ticket-triage-work/<ticket_id>/` in Finder
  - clicking a downloaded asset opens that file in Finder/default app
  - skipped/missing assets are disabled and explain the reason in the status area
  - keyboard fallback should exist, for example `i` for assets and an action to open the folder
- [ ] Make the bottom command bar clickable for safe actions:
  - open in Zendesk
  - edit selected field
  - open assets folder
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
  - clicking a right-sidebar field opens the field edit modal with a text box and current value
  - submitting a field edit writes only the selected custom field
  - invalid field update errors keep the edit modal open with the typed value
  - clicking an asset/folder region calls the local opener with the expected path
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
- Full rich field widgets for every Zendesk field type in the first mouse pass.
- Replacing keyboard shortcuts or making mouse support mandatory.

## Done When

- Tickets can be changed by clicking rows in the queue.
- The visible common actions can be clicked.
- Ticket fields can be clicked, edited in a text box, confirmed, and updated in Zendesk.
- Asset rows/folders can be clicked to open the relevant local path in Finder.
- Mouse wheel scrolling works in the queue and ticket timeline.
- All existing keyboard shortcuts still work.
- Posting, merging, requester cleanup, status changes, and time writes still require explicit approval.
- Focused tests and `go test ./... && go vet ./...` pass.
