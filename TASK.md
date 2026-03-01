# Feature: Error Handling + Checkpoint Resume + Exit Codes

**Branch:** `feature/error-handling`
**Worktree:** `/home/pavlo/purge-error-handling/`

---

## Problem

Checkpoint infrastructure exists but resume-on-startup is not wired. Exit codes are defined but never used. Telegram delete has no checkpoint saving. Error categorization (skipped vs failed vs already-deleted) is incomplete.

---

## Tasks

### 1. Wire checkpoint resume detection

In `cmd/delete.go` (both `runDiscordDelete` and `runTelegramDelete`):
- [ ] Before scanning, call `checkpoint.Manager.Load()`
- [ ] If checkpoint exists and matches current platform + target:
  - Print: "Found checkpoint from {time}: {deleted_count} already deleted. Resume? [y/N]"
  - If yes: skip messages up to `last_processed_id`, set counters from checkpoint
  - If no: clear checkpoint, start fresh
- [ ] If checkpoint exists but filters/target don't match: warn and offer fresh start

### 2. Wire checkpoint save for Telegram delete

In `cmd/delete.go` `runTelegramDelete`:
- [ ] Create checkpoint manager (same as Discord path)
- [ ] Register signal handler
- [ ] Save checkpoint after each batch (not each message — Telegram batches 100)
- [ ] Clear checkpoint on completion

### 3. Fix exit codes

In `cmd/root.go` or per-command:
- [ ] Auth failure (`ErrAuth`, `ErrNotAuthorized`): `os.Exit(ExitAuthFailure)` (2)
- [ ] Partial failure (deleted > 0 but failed > 0): `os.Exit(ExitPartial)` (3)
- [ ] Full success: `os.Exit(ExitSuccess)` (0)
- [ ] General error: `os.Exit(ExitError)` (1)

In `internal/checkpoint/checkpoint.go` `RegisterSignalHandler`:
- [ ] Change `os.Exit(1)` to `os.Exit(130)` (ExitInterrupted)

### 4. Track "already deleted" separately

In `cmd/delete.go` Discord delete loop:
- [ ] When `DeleteMessage` returns nil and the message was 404 (already deleted), increment `skipped` not `deleted`
- [ ] This requires `DeleteMessage` to distinguish "deleted successfully" from "was already gone"
  
In `internal/discord/client.go` `DeleteMessage`:
- [ ] Return a sentinel value or bool indicating if it was a 404 (already deleted) vs 200/204 (actually deleted)
- [ ] E.g., return `(alreadyDeleted bool, err error)` or a custom `ErrAlreadyDeleted`

### 5. Insufficient permissions: skip + warn + continue

In `cmd/delete.go` Discord delete loop:
- [ ] When `DeleteMessage` returns `*ErrForbidden`: 
  - Log warning: "Skipping message {id} in #{channel}: insufficient permissions"
  - Increment `skipped` (not `failed`)
  - Continue to next message (don't abort)

In `cmd/scan.go` Discord scan:
- [ ] When channel search returns `*ErrForbidden`:
  - Log warning: "Skipping #{channel}: insufficient permissions"
  - Continue to next channel

### 6. Verbose per-message logging

In `cmd/delete.go` (both platforms):
- [ ] When `viper.GetBool("verbose")` is true:
  - Log each successful delete: "Deleted message {id} in #{channel} ({timestamp})"
  - Log each skip: "Skipped message {id} (already deleted)"
  - Log each failure: "Failed to delete {id}: {error}"
- [ ] Use `fmt.Fprintf(os.Stderr, ...)` for log lines (don't mix with progress bar on stdout)

---

## Acceptance Criteria

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] Ctrl+C during delete saves checkpoint, exits with code 130
- [ ] Next `purge delete` with same target detects checkpoint, offers resume
- [ ] Resuming skips already-processed messages
- [ ] Auth failures exit with code 2
- [ ] Partial failures (some messages failed) exit with code 3
- [ ] `--verbose` shows per-message delete/skip/fail lines
- [ ] Permission errors on individual messages are skipped (not fatal)
- [ ] 404 (already deleted) messages counted as "skipped" not "deleted"
