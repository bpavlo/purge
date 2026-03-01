# Feature: README + Polish

**Branch:** `feature/polish`
**Worktree:** `/home/pavlo/purge-polish/`

---

## Tasks

### 1. README.md

Create `README.md` with:

- [ ] Project title + one-line description
- [ ] What it does (scan, filter, archive, delete messages from Discord & Telegram)
- [ ] Features list (bullet points)
- [ ] Install methods:
  - `go install github.com/pavlo/purge@latest`
  - Binary download from GitHub Releases
  - `nix run github:bpavlo/purge`
- [ ] Quick start:
  - `purge auth discord` / `purge auth telegram`
  - `purge scan discord --server "My Server"`
  - `purge delete discord --server "Old Server" --dry-run`
  - `purge delete telegram --chat "Group" --before 2025-01-01 --yes`
  - `purge archive discord --dms -o ~/backup/`
- [ ] Filter reference table (all 12 flags with descriptions)
- [ ] Configuration section (config file path, example snippet, env vars)
- [ ] Security & legal disclaimers:
  - Discord user tokens are against Discord ToS — personal use, self-hosted, user assumes risk
  - Telegram MTProto is officially documented and within their terms
  - No telemetry, no data leaves your device
  - Token storage details (local, 0600 permissions)
- [ ] License (MIT)
- [ ] Placeholder for demo GIF/asciicast (just a comment/TODO line)

### 2. Fix Nix flake vendorHash

In `flake.nix`:
- [ ] Run `nix build 2>&1` to get the real vendorHash (if Nix available)
- [ ] If Nix not available: add a comment explaining how to get it:
  ```
  # Run `nix build` once, it will fail and print the correct hash.
  # Replace lib.fakeHash with that value.
  ```
- [ ] Verify `nix build` succeeds if Nix is available

### 3. Clean up SPEC.md

- [ ] Either remove `SPEC.md` from the repo (it's internal planning) or move it to `docs/SPEC.md`
- [ ] If keeping: add a note at the top that it's the original design doc, not user-facing

---

## Acceptance Criteria

- [ ] `README.md` exists with install instructions, usage examples, and disclaimers
- [ ] README accurately reflects implemented features (not aspirational)
- [ ] `flake.nix` has real vendorHash or clear instructions
- [ ] `go build ./...` still passes
