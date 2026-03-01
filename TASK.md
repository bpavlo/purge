# Feature: Config Wiring + Rate Limit Fixes

**Branch:** `feature/config-ratelimit`
**Worktree:** `/home/pavlo/purge-config-ratelimit/`

---

## Problem

Rate limit defaults are dangerously fast (50ms) instead of SPEC values (1000ms Discord, 500ms Telegram). Several config values defined in `purge.yaml` are never read or applied.

---

## Tasks

### 1. Fix rate limit defaults

In `internal/ratelimit/limiter.go`:
- [ ] Change `DefaultConfig()` to return `DelayMs: 1000` (SPEC SS7 default for Discord)
- [ ] Add `DefaultTelegramConfig()` returning `DelayMs: 500, BatchSize: 100`

### 2. Wire rate limit config from viper

In `cmd/helpers.go` (or new file `cmd/config.go`):
- [ ] Add `discordRateLimitConfig()` that reads from viper:
  - `rate_limits.discord.delay_ms` (default 1000)
  - `rate_limits.discord.max_retries` (default 5)
  - `rate_limits.discord.backoff_multiplier` (default 2.0)
- [ ] Add `telegramRateLimitConfig()` that reads from viper:
  - `rate_limits.telegram.delay_ms` (default 500)
  - `rate_limits.telegram.batch_size` (default 100)
  - `rate_limits.telegram.max_retries` (default 5)
  - `rate_limits.telegram.backoff_multiplier` (default 2.0)
- [ ] Use these in `cmd/scan.go`, `cmd/delete.go`, `cmd/archive.go` when creating rate limiters

### 3. Wire config defaults for command behavior

In `cmd/delete.go`:
- [ ] Read `defaults.dry_run` from viper as fallback when `--dry-run` flag not set
- [ ] Read `defaults.archive_before_delete` from viper as fallback for `--archive`
- [ ] Read `defaults.exclude_pinned` from viper as fallback for `--exclude-pinned`

### 4. Support Discord token from config/env

In `cmd/helpers.go` `loadDiscordToken()`:
- [ ] Check `viper.GetString("discord.token")` first
- [ ] Then check env var `PURGE_DISCORD_TOKEN`
- [ ] Then fall back to file `~/.config/purge/discord_token`
- [ ] Document precedence in help text

### 5. Rate limit pause display

In `cmd/delete.go` (Discord delete loop):
- [ ] When `DeleteMessage` returns rate limit error or pauses, call `progressBar.SetStatus("Rate limited — pausing Xs...")`
- [ ] After pause completes, restore normal description

In `internal/discord/client.go`:
- [ ] Return a typed indicator when a 429 was handled (so the caller can show the pause)
- [ ] Or: add a callback/channel that fires on rate limit events

### 6. Rate limit debug logging

In `internal/discord/client.go` `doRequest()`:
- [ ] When 429 received: log at debug level "Rate limited on {method} {path}, waiting {duration}"
- [ ] Use `log/slog` with the configured log level from viper

In `internal/ratelimit/limiter.go`:
- [ ] When `HandleRateLimit` called: log at debug level

### 7. Update example config

In `configs/purge.example.yaml`:
- [ ] Add `defaults.dry_run`, `defaults.archive_before_delete` keys
- [ ] Add `rate_limits.discord.backoff_multiplier`, `rate_limits.telegram.backoff_multiplier`
- [ ] Ensure all keys match SPEC SS7 and SS8 exactly

---

## Acceptance Criteria

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] Default Discord delay is 1000ms, Telegram is 500ms
- [ ] `purge.yaml` rate limit values override defaults
- [ ] `PURGE_DISCORD_TOKEN` env var works for auth
- [ ] `defaults.exclude_pinned: true` in config applies without `--exclude-pinned` flag
- [ ] Rate limit pauses are visible in progress bar output
- [ ] Debug log shows rate limit events when `--log-level debug`
