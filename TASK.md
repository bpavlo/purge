# Feature: Discord Guild-Wide Search

**Branch:** `feature/discord`
**Worktree:** `/home/pavlo/purge-discord/`

---

## Problem

When scanning a Discord server, the code currently iterates every text channel and searches each one individually. Discord's API supports guild-wide search (`GET /guilds/{id}/messages/search?author_id={self}`) which is a single query. The `SearchGuildMessages` method already exists in `internal/discord/messages.go` but is unused in `cmd/scan.go`.

---

## Tasks

### 1. Use guild-wide search for server scans

In `cmd/scan.go` `runDiscordScan`:
- [ ] When `--server` is specified (without `--channel`):
  - Use `client.SearchGuildMessages()` instead of iterating channels
  - Paginate through all results (offset += 25 per page)
  - Group results by channel ID for the summary table
- [ ] When `--server` AND `--channel` are both specified:
  - Keep current behavior (search single channel)
- [ ] When `--dms`:
  - Keep current behavior (iterate DM channels, no guild search available)

### 2. Use guild-wide search for server deletes

In `cmd/delete.go` `runDiscordDelete`:
- [ ] Same logic: use guild-wide search when targeting a whole server
- [ ] Messages from the search already include `channel_id` — use that for `DeleteMessage` calls

### 3. Use guild-wide search for server archives

In `cmd/archive.go` `runDiscordArchive`:
- [ ] Same change: guild-wide search for full server archives
- [ ] Group archived messages by channel for output files

### 4. Handle search result grouping

Discord search returns messages in context groups (`[][]Message`). The `ExtractMessages(authorID)` helper already exists. Ensure:
- [ ] Results are correctly grouped by channel for summary table
- [ ] Pagination terminates when `offset >= total_results`
- [ ] Duplicate messages across pages are deduplicated (Discord search can overlap)

---

## Acceptance Criteria

- [ ] `purge scan discord --server "X"` uses one guild-wide search (not N channel searches)
- [ ] Results still show per-channel breakdown in the summary table
- [ ] `purge delete discord --server "X"` uses guild-wide search
- [ ] `--server "X" --channel "#general"` still searches only that channel
- [ ] `--dms` still works (iterates DM channels)
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
