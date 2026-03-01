> **Note:** This is the original design document. For usage, see [README.md](../README.md).

# purge — Mass Delete Messages from Discord & Telegram

> A CLI tool to scan, filter, archive, and bulk-delete your messages across Discord and Telegram.

---

## 1. Project Overview

**purge** is an open-source, self-hosted CLI tool inspired by [Redact](https://redact.dev/) that lets you mass-delete your own messages from Discord and Telegram. It runs entirely on your machine — no data leaves your device.

### Goals

- Delete your own messages in bulk from Discord DMs, group DMs, and servers
- Delete your own messages in bulk from Telegram private chats, groups, and channels
- Filter what gets deleted by date range, keyword, channel/chat, and content type
- Optionally archive messages locally (JSON) before deletion
- Dry-run mode so you can preview what would be deleted
- Clean CLI UX with progress bars, confirmation prompts, and summary stats
- Showcase-worthy on GitHub: good docs, CI, releases

### Non-Goals

- No GUI (CLI only, TUI stretch goal)
- No scheduled/recurring deletions (v1 — could be added via cron)
- No bot accounts — this uses **user tokens / user sessions** only
- No "edit-to-blank-then-delete" obfuscation tricks (just delete)
- No other platforms (Discord + Telegram only for v1)

---

## 2. Language Choice: Go

| Criteria              | Go | Python | TypeScript |
|-----------------------|----|--------|------------|
| Single static binary  | ✅  | ❌ (needs runtime) | ❌ (needs Node) |
| Concurrency model     | goroutines, native | asyncio, clunky | async/await, OK |
| CLI tooling ecosystem | excellent (cobra, bubbletea) | OK (click, rich) | OK (commander) |
| Cross-compile         | trivial (`GOOS/GOARCH`) | pyinstaller, messy | pkg, messy |
| GitHub appeal         | high for CLI tools | expected | unusual for CLI |
| Learning curve        | moderate, you'll be fine | trivial | trivial |

**Verdict:** Go gives you a single binary, great concurrency for rate-limited API calls, trivial cross-compilation for releases, and it's the natural language for polished CLI tools on GitHub. Your NixOS/infra background maps well to Go.

---

## 3. Architecture

```
purge/
├── cmd/                    # CLI entrypoints (cobra commands)
│   ├── root.go             # Global flags, config loading
│   ├── auth.go             # `purge auth discord|telegram`
│   ├── scan.go             # `purge scan discord|telegram`
│   ├── delete.go           # `purge delete discord|telegram`
│   └── archive.go          # `purge archive discord|telegram`
├── internal/
│   ├── discord/            # Discord API client + message types
│   │   ├── client.go       # HTTP client, auth, rate limiting
│   │   ├── messages.go     # Fetch, search, delete operations
│   │   └── types.go        # Discord message/channel structs
│   ├── telegram/           # Telegram client (MTProto via gotd/td)
│   │   ├── client.go       # Session management, auth flow
│   │   ├── messages.go     # Fetch, search, delete operations
│   │   └── types.go        # Telegram message/chat structs
│   ├── filter/             # Shared filtering logic
│   │   └── filter.go       # Date, keyword, content-type filters
│   ├── archive/            # Local JSON archiver
│   │   └── archive.go      # Write messages to JSON before delete
│   ├── ratelimit/          # Rate limiter (token bucket / per-endpoint)
│   │   └── limiter.go
│   └── ui/                 # CLI output helpers
│       └── progress.go     # Progress bars, spinners, tables
├── configs/
│   └── purge.example.yaml  # Example config file
├── go.mod
├── go.sum
├── main.go
├── Makefile
├── README.md
├── LICENSE                 # MIT
└── .goreleaser.yaml        # Automated releases
```

### Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config file + env vars |
| `github.com/charmbracelet/bubbletea` | Interactive TUI (stretch) |
| `github.com/charmbracelet/lipgloss` | Styled CLI output |
| `github.com/schollz/progressbar/v3` | Progress bars |
| `github.com/gotd/td` | Telegram MTProto client (user API) |
| `golang.org/x/time/rate` | Rate limiting |

> **Note on Discord:** There is no official Go SDK for Discord's **user** API. The tool will use raw HTTP calls to Discord's undocumented user endpoints (same approach Redact uses). A thin wrapper in `internal/discord/client.go` handles auth headers, rate limit response parsing, and retries.

---

## 4. Authentication

### 4.1 Discord

Discord does not offer OAuth for user-level message deletion. The tool requires a **user token** extracted from the browser (standard approach for self-use tools like this).

```
purge auth discord
```

**Flow:**
1. Prompt user to open Discord in browser → DevTools → Network tab → copy `Authorization` header
2. Store token in `~/.config/purge/discord_token` (file permissions `0600`)
3. Validate token by calling `/api/v9/users/@me`

**Security notes:**
- Token stored locally with restricted permissions
- Never logged or sent anywhere
- User warned about token sensitivity on setup

### 4.2 Telegram

Telegram requires a proper MTProto authentication via API ID + API Hash (obtained from https://my.telegram.org).

```
purge auth telegram
```

**Flow:**
1. Prompt for API ID and API Hash (one-time setup)
2. Initiate phone number + OTP login via MTProto (handled by `gotd/td`)
3. Session stored in `~/.config/purge/telegram_session` (encrypted by the library)
4. Validate by fetching current user info

---

## 5. Core Commands

### 5.1 `purge scan`

Scan and preview messages without deleting anything.

```bash
# Scan all your Discord messages in a specific server
purge scan discord --server "My Server"

# Scan Telegram messages in a specific chat, date-filtered
purge scan telegram --chat "John Doe" --after 2023-01-01 --before 2024-01-01

# Scan with keyword filter
purge scan discord --keyword "embarrassing" --server "Gaming"
```

**Output:** A summary table showing message counts by channel/chat, date range, and content types found.

```
 Channel/Chat         Messages  Date Range              Types
─────────────────────────────────────────────────────────────────
 #general             342       2022-03-15 → 2024-11-20  text, image, link
 #random              89        2023-01-02 → 2024-10-05  text, embed
 @username (DM)       1,204     2021-06-01 → 2024-12-01  text, image, file
─────────────────────────────────────────────────────────────────
 Total: 1,635 messages
```

### 5.2 `purge delete`

Delete messages matching the given filters. Always requires confirmation unless `--yes` is passed.

```bash
# Delete all your messages in a Discord server
purge delete discord --server "Old Server" --yes

# Delete Telegram messages older than 1 year
purge delete telegram --before 2025-02-28 --all-chats

# Dry run — show what WOULD be deleted
purge delete discord --server "Gaming" --dry-run

# Delete only messages containing a keyword
purge delete telegram --chat "Group Chat" --keyword "secret"

# Archive before deleting
purge delete discord --server "Work" --archive
```

**Confirmation prompt (unless `--yes`):**

```
⚠ You are about to delete 1,635 messages across 3 channels.
  Platform:  Discord
  Server:    Old Server
  Filters:   none (all messages)
  Archive:   disabled

  This action is IRREVERSIBLE.

  Type 'delete 1635' to confirm:
```

**Progress during deletion:**

```
Deleting messages... ████████████████████░░░░░░░░░░  67% (1,098/1,635)  ~4m remaining
  Rate limited — pausing 2.3s...
```

### 5.3 `purge archive`

Export messages to local JSON without deleting.

```bash
# Archive all Discord DMs
purge archive discord --dms --output ~/discord-archive/

# Archive a specific Telegram chat
purge archive telegram --chat "Family Group" --output ~/tg-backup/
```

**Output format** (`messages_general_2024.json`):

```json
{
  "platform": "discord",
  "exported_at": "2026-02-28T12:00:00Z",
  "channel": { "id": "123456", "name": "general", "server": "My Server" },
  "messages": [
    {
      "id": "789012",
      "timestamp": "2024-03-15T14:22:00Z",
      "content": "Hello world",
      "attachments": [],
      "type": "text"
    }
  ]
}
```

---

## 6. Filtering

All filters can be combined and apply to `scan`, `delete`, and `archive` commands.

| Flag | Description | Example |
|------|-------------|---------|
| `--server` | Discord server name or ID | `--server "Gaming"` |
| `--channel` | Discord channel name or ID | `--channel "#general"` |
| `--chat` | Telegram chat name or ID | `--chat "John Doe"` |
| `--dms` | Target all DMs (Discord) or private chats (Telegram) | `--dms` |
| `--all-chats` | Target everything accessible | `--all-chats` |
| `--after` | Messages after date (inclusive) | `--after 2024-01-01` |
| `--before` | Messages before date (inclusive) | `--before 2025-01-01` |
| `--keyword` | Messages containing text (case-insensitive) | `--keyword "oops"` |
| `--has-attachment` | Only messages with attachments | `--has-attachment` |
| `--has-link` | Only messages containing URLs | `--has-link` |
| `--min-length` | Messages with at least N characters | `--min-length 100` |
| `--exclude-pinned` | Skip pinned messages | `--exclude-pinned` |

---

## 7. Rate Limiting Strategy

Both APIs aggressively rate-limit. The tool must handle this gracefully.

### Discord

- Discord's user API returns `429` with a `Retry-After` header (seconds)
- Implement per-route rate limiting (different buckets for different endpoints)
- Default delay: **~1 request per second** for deletions (configurable)
- On 429: wait the specified time, then retry (up to 5 retries)
- Log rate limit events at debug level

### Telegram

- MTProto has `FLOOD_WAIT` errors with a seconds-to-wait value
- `gotd/td` handles basic flood wait internally
- Add additional backoff layer: default **~0.5s between deletions**
- Telegram allows deleting messages in batches (up to 100 IDs per `messages.deleteMessages` call) — use this for massive speedup
- Respect per-chat and global limits

### Config

```yaml
# ~/.config/purge/purge.yaml
rate_limits:
  discord:
    delay_ms: 1000          # ms between delete requests
    max_retries: 5
    backoff_multiplier: 2.0
  telegram:
    delay_ms: 500
    batch_size: 100          # messages per batch delete call
    max_retries: 5
    backoff_multiplier: 2.0
```

---

## 8. Configuration

Config file: `~/.config/purge/purge.yaml` (also supports env vars with `PURGE_` prefix via viper).

```yaml
# Default archive directory
archive_dir: ~/purge-archives

# Default behavior
defaults:
  dry_run: false
  archive_before_delete: false
  exclude_pinned: true

# Rate limit overrides (see §7)
rate_limits:
  discord:
    delay_ms: 1000
  telegram:
    delay_ms: 500
    batch_size: 100

# Logging
log_level: info   # debug | info | warn | error
log_file: ""      # empty = stderr only
```

---

## 9. Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid/expired token | Clear error message, prompt to re-auth |
| Rate limited (429 / FLOOD_WAIT) | Auto-retry with backoff, log at debug |
| Network failure mid-deletion | Save progress, resume on next run |
| Message already deleted | Skip silently, increment "already deleted" counter |
| Insufficient permissions | Skip channel/chat, warn user, continue others |
| Partial failure | Report summary: "Deleted 1,500. Failed 12. Skipped 3." |

### Resumability

On interruption (Ctrl+C or crash), write a checkpoint file:

```json
{
  "operation": "delete",
  "platform": "discord",
  "server_id": "123456",
  "last_processed_id": "789012",
  "deleted_count": 1500,
  "started_at": "2026-02-28T10:00:00Z"
}
```

On next run with the same filters, detect the checkpoint and offer to resume.

---

## 10. Platform-Specific Notes

### Discord Limitations

- **You can only delete your own messages.** Server messages sent by you are deletable regardless of channel permissions (via user API).
- **No bulk delete endpoint for user tokens.** Must delete one-by-one via `DELETE /api/v9/channels/{id}/messages/{id}`.
- **Search:** Use `GET /api/v9/guilds/{id}/messages/search?author_id={self}` for server-wide search, paginated.
- **DMs:** List DM channels via `GET /api/v9/users/@me/channels`, then fetch messages per channel.
- Discord user API is **undocumented and unofficial** — endpoint behavior may change without notice.

### Telegram Limitations

- **Private chats:** You can delete your own messages (and optionally for both sides via `revoke: true`).
- **Groups:** You can delete your own messages. Admins can delete anyone's messages.
- **Channels:** Only admins can delete messages.
- **48-hour rule:** For non-admin users, Telegram only allows deleting messages sent within the last 48 hours in some chat types. Older messages may fail silently.
- **Batch deletion:** `messages.deleteMessages` accepts up to 100 message IDs per call — significantly faster than Discord.
- **MTProto vs Bot API:** This tool uses MTProto (user login), not the Bot API, giving access to full chat history and user-level permissions.

---

## 11. CLI UX Polish

### Output Styles

- **Normal mode:** Progress bar + summary
- **Verbose (`-v`):** Per-message log lines
- **Quiet (`-q`):** Only errors and final summary
- **JSON output (`--json`):** Machine-readable output for scripting

### Colors & Formatting

Use `lipgloss` for styled output:
- Red for warnings/destructive actions
- Green for success/completion
- Yellow for rate-limit pauses
- Dim gray for debug info

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Auth failure |
| 3 | Partial failure (some messages failed to delete) |
| 130 | Interrupted (Ctrl+C), checkpoint saved |

---

## 12. GitHub & Distribution

### Release Pipeline

- **goreleaser** for cross-platform binaries (Linux, macOS, Windows)
- GitHub Actions CI: lint (`golangci-lint`), test, build on every PR
- Tagged releases auto-publish binaries + checksums
- Nix flake for `nix run github:user/purge` (you'll want this)

### Repo Polish

- `README.md` with demo GIF/asciicast, install instructions, usage examples
- `CONTRIBUTING.md`
- `LICENSE` (MIT)
- GitHub Issue templates
- Dependabot for dependency updates

### Install Methods

```bash
# Go install
go install github.com/user/purge@latest

# Homebrew (stretch)
brew install user/tap/purge

# Nix
nix run github:user/purge

# Binary download
curl -sSL https://github.com/user/purge/releases/latest/download/purge_linux_amd64 -o purge
chmod +x purge
```

---

## 13. Development Milestones

### v0.1 — Foundation

- [ ] Project scaffolding (cobra, viper, config)
- [ ] Discord auth flow (token input + validation)
- [ ] Discord scan command (list channels, fetch message counts)
- [ ] Discord delete command (single channel, basic filters)
- [ ] Dry-run mode
- [ ] Progress bar output

### v0.2 — Discord Complete

- [ ] All Discord filters (date, keyword, attachment, pinned)
- [ ] Discord DM scanning and deletion
- [ ] Server-wide search + delete
- [ ] Archive/export to JSON
- [ ] Rate limiting with retry logic
- [ ] Checkpoint/resume on interruption

### v0.3 — Telegram

- [ ] Telegram auth flow (API ID + OTP via MTProto)
- [ ] Telegram scan command
- [ ] Telegram delete command (batch mode)
- [ ] Telegram filters
- [ ] Telegram archive/export

### v0.4 — Polish

- [ ] Verbose/quiet/JSON output modes
- [ ] Config file support
- [ ] Comprehensive error handling
- [ ] goreleaser setup + GitHub Actions CI
- [ ] Nix flake
- [ ] README with demo, install instructions, usage guide

### Stretch Goals

- [ ] TUI mode with bubbletea (interactive channel/chat picker)
- [ ] `purge nuke` — delete everything, all platforms, no filters
- [ ] Cron-friendly mode for scheduled deletions
- [ ] Message edit history cleanup (Discord)
- [ ] Reaction removal (Discord)
- [ ] "Disappearing mode" — daemon that auto-deletes messages after N hours

---

## 14. Legal & Ethics

- This tool deletes **your own messages only** — it does not access or modify other users' content.
- Using user tokens (Discord) is against Discord's ToS. This is a personal-use, self-hosted tool. Users assume all risk. Document this clearly in the README.
- Telegram's MTProto user API is officially documented and using it for your own messages is within their terms.
- The tool never phones home, has no telemetry, and stores all data locally.
