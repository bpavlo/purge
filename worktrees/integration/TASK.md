# Worktree: Integration

**Phase:** 3 (after Discord + Telegram are merged)
**Branch:** `worktree/integration`
**Dependencies:** Foundation, Shared Core, Discord, Telegram

---

## Goal

Wire everything together end-to-end, ensure all commands work across both platforms, add consolidated tests, and polish error handling and output modes.

---

## Tasks

### 1. End-to-End Command Wiring

- [ ] Verify `purge auth discord` and `purge auth telegram` work independently
- [ ] Verify `purge scan discord` and `purge scan telegram` produce correct output
- [ ] Verify `purge delete` with `--dry-run` works for both platforms
- [ ] Verify `purge delete` with actual deletion works for both platforms
- [ ] Verify `purge archive` exports correct JSON for both platforms
- [ ] Verify all filter flags work across both platforms

### 2. Output Modes

- [ ] **Normal mode**: progress bar + summary (default)
- [ ] **Verbose (`-v`)**: per-message log lines during scan/delete
- [ ] **Quiet (`-q`)**: errors + final summary only, no progress bar
- [ ] **JSON (`--json`)**: all output as JSON objects (one per line or structured)
  - Scan output: `{"channels": [...], "total": N}`
  - Delete output: `{"deleted": N, "failed": N, "skipped": N}`
  - Archive output: `{"files": ["path1.json", "path2.json"]}`
- [ ] Ensure modes are mutually exclusive where needed (`-v` and `-q` conflict)

### 3. Confirmation Prompt Polish

- [ ] Format matches SPEC section 5.2 exactly:
  ```
  Warning: You are about to delete 1,635 messages across 3 channels.
    Platform:  Discord
    Server:    Old Server
    Filters:   none (all messages)
    Archive:   disabled

    This action is IRREVERSIBLE.

    Type 'delete 1635' to confirm:
  ```
- [ ] Correctly format message count with commas
- [ ] Show active filters in summary
- [ ] Show archive status
- [ ] `--yes` flag bypasses completely

### 4. Resumability End-to-End

- [ ] On Ctrl+C during delete: checkpoint saves with correct state
- [ ] On next `purge delete` with same filters: detect checkpoint, prompt to resume
- [ ] Resume skips already-deleted messages correctly
- [ ] After successful completion: checkpoint file is cleared
- [ ] Edge case: checkpoint exists but filters don't match — warn and offer fresh start

### 5. Error Handling

- [ ] Invalid/expired Discord token: clear message, suggest `purge auth discord`
- [ ] Invalid/expired Telegram session: clear message, suggest `purge auth telegram`
- [ ] Network failure mid-deletion: checkpoint saved, user informed
- [ ] Permission errors: skip chat/channel, warn, continue others
- [ ] Partial failure summary: "Deleted 1,500. Failed 12. Skipped 3."
- [ ] Exit codes:
  - 0: all succeeded
  - 1: general error
  - 2: auth failure
  - 3: partial failure (some deletes failed)
  - 130: interrupted, checkpoint saved

### 6. Consolidated Tests

Tests should cover integration points, not duplicate unit tests from shared-core.

- [ ] **Filter integration test**: create mock messages, apply filters, verify results
- [ ] **Archive integration test**: scan mock messages, archive, verify JSON output
- [ ] **Rate limiter integration test**: simulate rapid calls, verify delays
- [ ] **Checkpoint integration test**: simulate interruption, verify save/resume
- [ ] **CLI flag parsing test**: verify all flags parsed correctly for all commands
- [ ] **Discord client test**: mock HTTP responses, verify auth/search/delete flows
- [ ] **Telegram client test**: mock API responses where feasible (gotd has `tgtest` and `tgmock` packages)
- [ ] **Output mode test**: verify JSON output is valid JSON, quiet mode suppresses progress

### 7. Cross-Platform Edge Cases

- [ ] Server/chat that has zero messages from user — handle gracefully
- [ ] Very large message count (10k+) — verify pagination works end-to-end
- [ ] Messages with Unicode content — keyword filter works correctly
- [ ] Messages with no content (embed-only, attachment-only) — don't crash
- [ ] Deleted account in DM — handle gracefully

---

## Acceptance Criteria

- [ ] All commands work end-to-end for both platforms
- [ ] All 4 output modes produce correct output
- [ ] Confirmation prompt matches spec format
- [ ] Checkpoint save/resume works across interruptions
- [ ] Exit codes are correct for all scenarios
- [ ] Test suite passes: `go test ./...`
- [ ] No panics on edge cases (empty results, network errors, permission issues)

---

## Notes

- This worktree should NOT rewrite platform logic. If something is broken in discord/telegram packages, fix it there and merge.
- Focus on the seams: where CLI meets platform code, where platform code meets shared code.
- Test with real accounts if possible, but don't require it — mock-based tests are the priority.
- `gotd/td` provides `tgmock` for mocking Telegram API calls and `tgtest` for test server — use these.
