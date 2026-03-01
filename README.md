# purge

A CLI tool to mass-delete your own messages from Discord and Telegram.

Scan, filter, archive, and bulk-delete messages across platforms — entirely from your terminal. No data leaves your machine.

<!-- TODO: Add demo GIF or asciinema cast -->

## Features

- **Multi-platform** — Discord servers, DMs, and Telegram chats (private, groups, channels)
- **Powerful filters** — date range, keyword, attachments, links, message length, pinned status
- **Scan before you act** — preview matching messages without modifying anything
- **Archive/export** — save messages as JSON before (or instead of) deleting
- **Dry-run mode** — see exactly what would be deleted
- **Confirmation prompts** — type the count to confirm, or pass `--yes` to skip
- **Resumable** — checkpoints on interruption, resume where you left off
- **Rate-limit aware** — automatic backoff and retry for both platforms
- **Configurable** — YAML config file, env vars (`PURGE_` prefix), and CLI flags
- **Output modes** — normal (progress bar), verbose, quiet, and JSON for scripting

## Install

### Go

```bash
go install github.com/bpavlo/purge@latest
```

### Binary download

Download a prebuilt binary from [GitHub Releases](https://github.com/bpavlo/purge/releases) and place it in your `$PATH`.

### Nix

```bash
nix run github:bpavlo/purge
```

## Quick Start

Authenticate with your platform:

```bash
purge auth discord       # paste your Discord user token
purge auth telegram      # enter API ID, hash, and phone number
```

Scan to preview what you have:

```bash
purge scan discord --server "My Server"
purge scan telegram --chat "John Doe" --after 2024-01-01
```

Delete messages:

```bash
purge delete discord --server "Old Server" --dry-run          # preview first
purge delete discord --server "Old Server" --yes              # skip confirmation
purge delete telegram --chat "Group" --before 2025-01-01 --yes
```

Archive messages to a local directory:

```bash
purge archive discord --dms -o ~/backup/
purge archive telegram --chat "Family Group" -o ~/tg-backup/
```

## Filter Reference

All filters work with `scan`, `delete`, and `archive`.

| Flag | Description | Example |
|------|-------------|---------|
| `--server` | Discord server name or ID | `--server "Gaming"` |
| `--channel` | Discord channel name or ID | `--channel "general"` |
| `--chat` | Telegram chat name or ID | `--chat "John Doe"` |
| `--dms` | Target all DMs / private chats | `--dms` |
| `--all-chats` | Target all accessible chats | `--all-chats` |
| `--after` | Messages after date (inclusive, YYYY-MM-DD) | `--after 2024-01-01` |
| `--before` | Messages before date (inclusive, YYYY-MM-DD) | `--before 2025-01-01` |
| `--keyword` | Case-insensitive text match | `--keyword "oops"` |
| `--has-attachment` | Only messages with attachments | `--has-attachment` |
| `--has-link` | Only messages containing URLs | `--has-link` |
| `--min-length` | Minimum message length (characters) | `--min-length 100` |
| `--exclude-pinned` | Skip pinned messages | `--exclude-pinned` |

The `delete` command also accepts:

| Flag | Description |
|------|-------------|
| `--yes` | Skip confirmation prompt |
| `--dry-run` | Preview only, don't delete |
| `--archive` | Archive messages before deleting |

## Configuration

Config file location: `~/.config/purge/purge.yaml`

All settings can also be set via environment variables with the `PURGE_` prefix (e.g. `PURGE_DISCORD_TOKEN`).

Example config:

```yaml
discord:
  token: ""                  # or set PURGE_DISCORD_TOKEN

telegram:
  api_id: ""                 # or set PURGE_TELEGRAM_API_ID
  api_hash: ""               # or set PURGE_TELEGRAM_API_HASH
  phone: "+1234567890"

archive_dir: ~/purge-archive
archive_format: json
log_level: info

defaults:
  exclude_pinned: false
```

See [`configs/purge.example.yaml`](configs/purge.example.yaml) for the full annotated example.

## Security & Legal

**Discord:** Using user tokens is against [Discord's Terms of Service](https://discord.com/terms). This is a personal-use, self-hosted tool. You assume all risk. Your token is stored locally at `~/.config/purge/discord_token` with `0600` permissions and is never sent to any third party.

**Telegram:** The MTProto user API is [officially documented](https://core.telegram.org/api) by Telegram. Using it to manage your own messages is within their terms. Sessions are stored locally at `~/.config/purge/telegram_session`.

**Privacy:** Purge has no telemetry, no analytics, and no network calls beyond the platform APIs. All data stays on your device.

**Scope:** This tool only deletes **your own messages**. It does not access or modify other users' content.

## License

[MIT](LICENSE)
