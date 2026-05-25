# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Role-based feature gating — `zentui` now detects the authenticated user's Zendesk role (admin, agent, light agent) via `/api/v2/users/me` and proactively hides or blocks features the user cannot use
- Light agent restrictions enforced across CLI, TUI, and MCP server: public comments, status changes, ticket assignment, CCs, and ticket deletion are blocked with clear error messages
- TUI comment panel shows "Internal note only (light agent)" and hides the public/internal toggle and CC picker for light agents
- TUI command palette and status key binding are hidden for light agents
- MCP server validates operations against user permissions at tool invocation time, returning descriptive errors instead of letting Zendesk reject them
- CLI commands (`tickets update`, `tickets create`, `tickets delete`) gate restricted flags early with actionable error messages
- `--demo-role` flag for testing role-based behavior without authentication (`--demo --demo-role light_agent`)
- Public OAuth client support — `--client-secret` is no longer required when using a Zendesk public OAuth client (secured via PKCE, which was already implemented)
- OAuth refresh token support — access tokens auto-refresh transparently in the HTTP transport layer when a refresh token is available
- `auth status` now shows token expiry time, validity status, and whether auto-refresh is enabled

### Changed

- `User` type now includes `role_type`, `restricted_agent`, and `custom_role_id` fields from the Zendesk API
- Light agents' comments default to internal notes (CLI silently overrides `--public` default; MCP server sets `public: false`)
- OAuth login flow returns structured `OAuthResult` (access token, refresh token, expiry) instead of a bare token string
- Simplified agent onboarding — after an admin configures `subdomain` and `oauth_client_id` once, agents authenticate with just `zentui auth login`

## [0.5.0] - 2026-03-13

### Changed

- Upgraded TUI framework to charm.land v2 ecosystem: Bubble Tea v2, Bubbles v2, Lip Gloss v2, Glamour v2
- TUI now uses declarative View rendering (alt screen, window title) instead of imperative commands
- Terminal theme detection via `lipgloss/v2/compat` AdaptiveColor for light/dark mode support
- Migrated all 26 test files to `stretchr/testify` (assert/require), reducing test boilerplate by ~870 lines
- Command palette is wider and taller for better readability

### Fixed

- Command palette inner width now accounts for border sizing in Lip Gloss v2
- Command palette shortcut filtering — typing `/`, `g`, etc. now matches commands by shortcut when fuzzy search has no results

## [0.4.9] - 2026-03-12

### Added

- In-memory TTL cache for TUI API calls — avoids redundant `Get`, `ListAudits`, `ListComments`, `List`, and `Search` requests within a 60-second window
- Cursor settle debounce (300ms) in split view — detail panel only loads after the user stops scrolling, preventing wasted API calls during rapid navigation
- Cache invalidation on mutations — `Create`, `Update`, and `Delete` immediately clear stale ticket and search cache entries

### Fixed

- Styled `zentui` logo in TUI header
- Newlines in ticket subjects no longer break list and kanban row rendering

## [0.4.8] - 2026-03-12

### Added

- Shell completion command (`zentui completion [bash|zsh|fish|powershell]`) with install instructions for each shell
- Man page generation (`zentui man`) using `cobra/doc` — hidden command used during release builds
- FreeBSD release binaries (amd64, arm64)
- 386 and armv7 release binaries for Linux and Windows
- APK packages for Alpine Linux
- Scoop bucket for Windows (`scoop install zentui`)
- Shell completions (bash/zsh/fish) bundled in release archives and installed via Homebrew/deb/rpm/apk
- Man pages bundled in release archives and installed via Homebrew/deb/rpm/apk
- CI build workflow running tests on Ubuntu, macOS, and Windows for every push and PR

### Changed

- Release archive naming uses human-friendly platform names (macOS, x86_64, i386) instead of Go identifiers
- Changelog in GitHub releases now groups commits by type (New Features, Bug Fixes, Performance, etc.)

## [0.4.7] - 2026-03-11

### Added

- Audit timeline in TUI detail view — replaces flat comments with a vertical timeline showing field changes (status, priority, assignee) alongside comments with connector lines
- Timeline filter — press `f` to toggle between all events and comments-only
- Text wrapping for long URLs and unbroken text in description and timeline panels

### Fixed

- Detail panel now shows ticket top (Details + Description) on load instead of scrolling to bottom

## [0.4.6] - 2026-03-11

### Added

- "My tickets" shortcut in TUI — press `m` to toggle a filter showing only tickets assigned to you; press `m` again or `esc` to clear
- "My tickets" entry in command palette

## [0.4.5] - 2026-03-09

### Added

- Image attachment support in TUI — inline indicators (📷/📎) below comments show attachments with filename and size
- Image picker overlay — press `i` in detail view to browse image attachments and open them in the default system app
- Extended `Attachment` type with `inline`, `width`, `height`, and `thumbnails` fields from Zendesk API
- Demo mode now generates sample image attachments on ~25% of comments

### Fixed

- CC picker text input swallowing `j`, `k` and other vim-bound keys — arrow-only bindings now used for result navigation so all letters type correctly

## [0.4.4] - 2026-03-09

### Fixed

- Kanban board right-side padding clipping — rightmost column no longer overflows the terminal edge

## [0.4.3] - 2026-03-09

### Added

- Kanban board view in TUI — toggle with `w` to group tickets by status into columns
- Responsive column layout adapting from 5 columns down to 1 based on terminal width
- Per-column scrolling with scroll indicators
- Cursor preservation across data refreshes
- Left/Right (`h`/`l`) and Up/Down (`j`/`k`) navigation for kanban
- Kanban toggle in command palette

### Fixed

- Use valid go version format in go.mod

## [0.4.2] - 2026-03-08

### Added

- Command palette with Ctrl+P
- Toggleable tags column
- Build version in footer next to email
- Linux deb/rpm packages via nfpm

### Fixed

- Newest comments shown first in detail views
- Chart anchored to bottom with improved spacing
- Equal-width panels in split view

## [0.4.1] - 2026-03-08

### Added

- Window title
- Bell notifications
- Ticket status chart in TUI

## [0.4.0] - 2026-03-07

### Added

- Built-in MCP server (`zentui mcp serve`)

### Security

- Hardened auth, HTTP transport, credentials, and error handling

## [0.3.0] - 2026-03-07

### Added

- Natural language to Zendesk search query translation
- Split-panel TUI with detail side panel
- Infinite scroll pagination
- Go-to-ticket shortcut (g)
- `--demo` flag for offline showcase
- User status bar and open-in-browser
- Ticket comments and Help Center articles commands
- Hour-level time support in NLQ

### Fixed

- Clear detail panel on empty results
- Nil map panic
- Export search 422 error

## [0.2.0] - 2026-03-07

### Added

- Interactive TUI mode via `zentui tui`
- Auto-refresh with countdown and manual refresh
- User sideloading to ticket commands

## [0.1.0] - 2026-03-06

### Added

- Initial release — Zendesk CLI with ticket CRUD, search, auth (OAuth + API token), JSON/text/NDJSON output, field projection, retry with backoff, and profile support

[Unreleased]: https://github.com/itsolver/zentui/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/itsolver/zentui/compare/v0.4.9...v0.5.0
[0.4.9]: https://github.com/itsolver/zentui/compare/v0.4.8...v0.4.9
[0.4.8]: https://github.com/itsolver/zentui/compare/v0.4.7...v0.4.8
[0.4.7]: https://github.com/itsolver/zentui/compare/v0.4.6...v0.4.7
[0.4.6]: https://github.com/itsolver/zentui/compare/v0.4.5...v0.4.6
[0.4.5]: https://github.com/itsolver/zentui/compare/v0.4.4...v0.4.5
[0.4.4]: https://github.com/itsolver/zentui/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/itsolver/zentui/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/itsolver/zentui/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/itsolver/zentui/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/itsolver/zentui/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/itsolver/zentui/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/itsolver/zentui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/itsolver/zentui/releases/tag/v0.1.0
