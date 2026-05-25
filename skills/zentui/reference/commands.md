# Command Reference

Complete reference for every `zentui` command and flag. For the most up-to-date info, run `zentui commands -o json`.

## Global flags

These flags are available on all commands:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--output` | `-o` | string | `text` | Output format: `text`, `json`, `ndjson` |
| `--fields` | | stringSlice | | Field projection (comma-separated) |
| `--no-headers` | | bool | `false` | Omit table headers in text mode |
| `--non-interactive` | | bool | `false` | Never prompt for input |
| `--yes` | | bool | `false` | Auto-confirm prompts |
| `--trace-id` | | string | | Trace ID attached to API requests |
| `--subdomain` | | string | | Override Zendesk subdomain |
| `--profile` | | string | `default` | Config profile |
| `--demo` | | bool | `false` | Use synthetic demo data (no auth required) |

## auth

### `zentui auth login`

Authenticate with Zendesk.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--method` | string | `oauth` | Authentication method: `oauth` or `token` |
| `--email` | string | | Zendesk email (required for token auth) |
| `--api-token` | string | | Zendesk API token (required for token auth) |
| `--subdomain` | string | | Zendesk subdomain (required) |
| `--client-id` | string | | OAuth client ID (required for first-time OAuth) |
| `--client-secret` | string | | OAuth client secret (required for first-time OAuth) |
| `--scope` | string | `read write` | OAuth scope |

**Token auth example:**

```bash
zentui auth login --method token --subdomain acme --email agent@acme.com --api-token TOKEN
```

**OAuth example:**

```bash
zentui auth login --subdomain acme --client-id ID --client-secret SECRET
```

### `zentui auth logout`

Remove stored credentials for the current profile.

No additional flags.

### `zentui auth status`

Show authentication status. Returns profile, method, subdomain, and email.

No additional flags. Exit code 3 if not authenticated.

## tickets

### `zentui tickets list`

List tickets with optional filtering.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `100` | Maximum number of tickets to return |
| `--cursor` | string | | Pagination cursor |
| `--sort` | string | `updated_at` | Sort field |
| `--sort-order` | string | `desc` | Sort order: `asc` or `desc` |
| `--status` | string | | Filter by status |
| `--assignee` | int64 | | Filter by assignee ID |
| `--group` | int64 | | Filter by group ID |
| `--include` | string | | Sideload related records: `users`, `groups`, `organizations` |

**Default output columns:** `id`, `status`, `priority`, `subject`, `updated_at`

When `--include users` is used, columns change to: `id`, `status`, `priority`, `requester_name`, `assignee_name`, `subject`, `updated_at`. Enriched fields available for `--fields`: `requester_name`, `requester_email`, `assignee_name`, `assignee_email`.

```bash
zentui tickets list -o json --limit 50 --status open --sort-order asc
zentui tickets list -o json --include users --fields id,subject,requester_name,assignee_name
```

### `zentui tickets show <id>`

Show a single ticket by ID.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--include` | string | | Sideload related records: `users`, `groups`, `organizations` |

**Positional argument:** `id` (required) — ticket ID

When `--include users` is used, output is enriched with `requester_name`, `requester_email`, `assignee_name`, `assignee_email`.

```bash
zentui tickets show 12345 -o json --include users
zentui tickets show 12345 -o json --include users --fields id,subject,requester_name,requester_email
```

### `zentui tickets create`

Create a new ticket.

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--subject` | string | | **yes** | Ticket subject |
| `--comment` | string | | **yes** | Ticket comment body |
| `--priority` | string | | | Priority: `urgent`, `high`, `normal`, `low` |
| `--type` | string | | | Type: `problem`, `incident`, `question`, `task` |
| `--status` | string | | | Status: `new`, `open`, `pending`, `hold`, `solved`, `closed` |
| `--assignee-id` | int64 | | | Assignee user ID |
| `--group-id` | int64 | | | Group ID |
| `--tags` | stringSlice | | | Tags (comma-separated) |
| `--custom-field` | stringArray | | | Custom field (`id=value`, repeatable) |
| `--requester-email` | string | | | Requester email |
| `--requester-name` | string | | | Requester name |
| `--idempotency-key` | string | | | Idempotency key for deduplication |
| `--if-exists` | string | `error` | | When idempotent ticket exists: `skip`, `update`, `error` |

```bash
zentui tickets create -o json --subject "Bug report" --comment "Steps to reproduce..." --priority high --tags bug,frontend
```

### `zentui tickets update <id>`

Update an existing ticket. Only changed flags are sent.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--subject` | string | | New subject |
| `--comment` | string | | Comment body |
| `--public` | bool | `true` | Whether comment is public |
| `--priority` | string | | Priority: `urgent`, `high`, `normal`, `low` |
| `--status` | string | | Status: `new`, `open`, `pending`, `hold`, `solved`, `closed` |
| `--assignee-id` | int64 | | Assignee user ID |
| `--group-id` | int64 | | Group ID |
| `--tags` | stringSlice | | Replace all tags |
| `--add-tags` | stringSlice | | Add tags |
| `--remove-tags` | stringSlice | | Remove tags |
| `--cc` | stringSlice | | Add CCs (emails or user IDs, comma-separated) |
| `--custom-field` | stringArray | | Custom field (`id=value`, repeatable) |
| `--safe-update` | bool | `false` | Use safe update (conflict detection) |

**Positional argument:** `id` (required) — ticket ID

```bash
zentui tickets update 12345 -o json --status pending --comment "Waiting on customer" --public=false
```

### `zentui tickets delete <id>`

Delete a ticket with two-step safety confirmation.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Preview deletion and return confirmation ID |
| `--confirm` | string | | Execute deletion with confirmation ID from dry-run |
| `--yes` | bool | `false` | Skip two-step confirmation |

**Positional argument:** `id` (required) — ticket ID

```bash
# Safe two-step delete
zentui tickets delete 12345 -o json --dry-run
zentui tickets delete 12345 -o json --confirm abc123def456

# Direct delete (skip confirmation)
zentui tickets delete 12345 -o json --yes
```

### `zentui tickets search <query>`

Search tickets using Zendesk search syntax.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `100` | Maximum number of results |
| `--cursor` | string | | Pagination cursor |
| `--sort-by` | string | | Sort field |
| `--sort-order` | string | `desc` | Sort order: `asc` or `desc` |
| `--export` | bool | `false` | Use export endpoint for >1000 results |
| `--include` | string | | Sideload related records: `users`, `groups`, `organizations` |

**Positional argument:** `query` (required) — Zendesk search query string

**Default output columns:** `id`, `status`, `priority`, `subject`, `updated_at`

When `--include users` is used, columns change to: `id`, `status`, `priority`, `requester_name`, `assignee_name`, `subject`, `updated_at`.

```bash
zentui tickets search "status:open priority:urgent" -o json --limit 50
zentui tickets search "status:open" -o json --include users
```

### `zentui tickets comments <id>`

List comments on a ticket.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `100` | Maximum number of comments to return |
| `--cursor` | string | | Pagination cursor |
| `--sort-order` | string | `asc` | Sort order: `asc` or `desc` |
| `--include` | string | | Sideload related records: `users` |

**Positional argument:** `id` (required) — ticket ID

```bash
zentui tickets comments 12345 -o json
zentui tickets comments 12345 -o json --sort-order desc --limit 50
zentui tickets comments 12345 -o json --include users
```

## articles

### `zentui articles list`

List Help Center articles.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `25` | Maximum number of articles to return |
| `--cursor` | string | | Pagination cursor |
| `--sort-by` | string | | Sort field: `title`, `created_at`, `updated_at` |
| `--sort-order` | string | `desc` | Sort order: `asc` or `desc` |

```bash
zentui articles list -o json
zentui articles list -o json --limit 50 --sort-by updated_at --sort-order asc
```

### `zentui articles show <id>`

Show a single Help Center article by ID.

**Positional argument:** `id` (required) — article ID

```bash
zentui articles show 360001234567 -o json
```

### `zentui articles search <query>`

Search Help Center articles.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `25` | Maximum number of results |
| `--cursor` | string | | Pagination cursor |

**Positional argument:** `query` (required) — search query string

```bash
zentui articles search "password reset" -o json
zentui articles search "billing" -o json --limit 10
```

## config

### `zentui config show`

Show current configuration for the active profile.

No additional flags.

### `zentui config set <key> <value>`

Set a configuration value for the active profile.

**Positional arguments:** `key` (required), `value` (required)

```bash
zentui config set subdomain acme --profile production
```

## Utility commands

### `zentui commands`

List all available commands with their flags. Use `-o json` for machine-readable output for agent discovery.

```bash
zentui commands -o json
```

### `zentui schema`

Output JSON Schema for a command's input, for AI agent tool calling.

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--command` | string | | **yes** | Command name (e.g., `tickets create`) |

```bash
zentui schema --command "tickets create"
zentui schema --command "tickets search"
```

### `zentui tui`

Launch the interactive terminal UI for browsing and managing tickets.

No additional flags (uses global flags like `--demo`, `--profile`).

```bash
zentui tui
zentui tui --demo
```

### `zentui mcp serve`

Start an MCP (Model Context Protocol) server on stdio for AI agent integration.

No additional flags. Uses the same authentication as the CLI.

```bash
zentui mcp serve
zentui mcp serve --demo
```

### `zentui completion`

Generate shell autocompletion scripts.

Available subcommands: `bash`, `fish`, `powershell`, `zsh`.

```bash
zentui completion bash
zentui completion zsh
```

### `zentui version`

Print version, commit hash, and build date.
