# Worktree: Shared Core

**Phase:** 1 (start alongside foundation)
**Branch:** `worktree/shared-core`
**Dependencies:** Foundation (for types/structs), but can be developed in parallel with agreed interfaces

---

## Goal

Build all platform-agnostic internal packages: filtering, archiving, rate limiting, UI output, and checkpoint/resume. These are consumed by both Discord and Telegram worktrees.

---

## Tasks

### 1. Message Types (`internal/types/types.go`)

Define a unified message representation used across platforms:

- [ ] `Message` struct:
  ```go
  type Message struct {
      ID          string
      Platform    string    // "discord" or "telegram"
      ChannelID   string
      ChannelName string
      ServerID    string    // Discord only
      ServerName  string    // Discord only
      ChatID      string    // Telegram only
      ChatName    string    // Telegram only
      Content     string
      Timestamp   time.Time
      Type        string    // "text", "image", "file", "embed", "link"
      HasAttachment bool
      HasLink     bool
      IsPinned    bool
      Attachments []Attachment
  }
  ```
- [ ] `Attachment` struct (filename, URL, size, content type)
- [ ] `Channel` / `Chat` struct for listing targets
- [ ] `ScanResult` struct (channel info, message count, date range, types found)

### 2. Filter Package (`internal/filter/filter.go`)

- [ ] `FilterOptions` struct matching all CLI flags:
  - `After`, `Before` (time.Time)
  - `Keyword` (string, case-insensitive)
  - `HasAttachment`, `HasLink` (bool)
  - `MinLength` (int)
  - `ExcludePinned` (bool)
- [ ] `Filter` function: `func Match(msg *types.Message, opts FilterOptions) bool`
- [ ] Each filter condition is a separate check, combined with AND logic
- [ ] Keyword matching is case-insensitive (`strings.Contains` with `strings.ToLower`)
- [ ] Date comparison: `After` is inclusive (>=), `Before` is inclusive (<=)
- [ ] Unit tests for each filter type and combinations

### 3. Archive Package (`internal/archive/archive.go`)

- [ ] `Archiver` struct with output directory
- [ ] `Archive(messages []types.Message, metadata ArchiveMetadata) error`
- [ ] Output format matches SPEC section 5.3:
  ```json
  {
    "platform": "discord",
    "exported_at": "2026-02-28T12:00:00Z",
    "channel": { "id": "...", "name": "...", "server": "..." },
    "messages": [...]
  }
  ```
- [ ] File naming: `messages_{channelname}_{date}.json`
- [ ] Create output directory if it doesn't exist
- [ ] Handle file write errors gracefully
- [ ] Unit tests

### 4. Rate Limiter (`internal/ratelimit/limiter.go`)

- [ ] `RateLimiter` struct configurable per SPEC section 7:
  - `DelayMs` — milliseconds between requests
  - `MaxRetries` — max retry count
  - `BackoffMultiplier` — exponential backoff factor
  - `BatchSize` — for Telegram batch deletes
- [ ] `Wait(ctx context.Context) error` — blocks until next request is allowed
- [ ] `HandleRateLimit(retryAfter time.Duration)` — called when API returns 429/FLOOD_WAIT
- [ ] Support per-route buckets (Discord has different limits per endpoint)
- [ ] Use `golang.org/x/time/rate` as the underlying token bucket
- [ ] Respect context cancellation
- [ ] Unit tests

### 5. UI Package (`internal/ui/progress.go`)

- [ ] `ProgressBar` wrapper around `schollz/progressbar/v3`:
  - `NewProgressBar(total int, description string) *ProgressBar`
  - `Increment()`
  - `Finish()`
  - `SetStatus(msg string)` — for rate limit pauses
- [ ] `SummaryTable` — format scan results as table (SPEC section 5.1):
  ```
   Channel/Chat         Messages  Date Range              Types
  ─────────────────────────────────────────────────────────────────
   #general             342       2022-03-15 → 2024-11-20  text, image
  ```
- [ ] `ConfirmationPrompt` — for delete command (SPEC section 5.2):
  - Show warning with count, platform, filters, archive status
  - Require typing "delete {count}" to confirm
  - Return error if user cancels
- [ ] `DeleteSummary` — final report: "Deleted X. Failed Y. Skipped Z."
- [ ] Use `lipgloss` for colors:
  - Red: warnings/destructive
  - Green: success
  - Yellow: rate limit pauses
  - Dim gray: debug
- [ ] Respect `--quiet` (no progress bar, only errors + summary)
- [ ] Respect `--json` (output JSON instead of styled text)

### 6. Checkpoint/Resume (`internal/checkpoint/checkpoint.go`)

- [ ] `Checkpoint` struct per SPEC section 9:
  ```go
  type Checkpoint struct {
      Operation       string    `json:"operation"`
      Platform        string    `json:"platform"`
      ServerID        string    `json:"server_id,omitempty"`
      ChatID          string    `json:"chat_id,omitempty"`
      LastProcessedID string    `json:"last_processed_id"`
      DeletedCount    int       `json:"deleted_count"`
      FailedCount     int       `json:"failed_count"`
      SkippedCount    int       `json:"skipped_count"`
      StartedAt       time.Time `json:"started_at"`
  }
  ```
- [ ] `Save(checkpoint Checkpoint) error` — write to `~/.config/purge/checkpoint.json`
- [ ] `Load() (*Checkpoint, error)` — read existing checkpoint
- [ ] `Clear() error` — remove checkpoint file after successful completion
- [ ] `Exists() bool` — check if a checkpoint exists for resume prompt
- [ ] Register signal handler for SIGINT (Ctrl+C) to save checkpoint before exit
- [ ] Unit tests

---

## Acceptance Criteria

- [ ] All packages compile independently with `go build ./internal/...`
- [ ] Unit tests pass: `go test ./internal/filter/ ./internal/archive/ ./internal/ratelimit/ ./internal/checkpoint/`
- [ ] Filter correctly handles all flag combinations
- [ ] Archive produces valid JSON matching the spec format
- [ ] Rate limiter respects delays and handles backoff
- [ ] Progress bar renders and updates correctly
- [ ] Checkpoint saves/loads/clears correctly

---

## Notes

- The `internal/types` package is the glue between platforms and shared logic. Discord and Telegram worktrees will convert their platform-specific types into `types.Message`.
- Don't over-abstract. These are internal packages, not a public API.
- The UI package should degrade gracefully if stdout is not a TTY (no colors, no progress bar animations).
