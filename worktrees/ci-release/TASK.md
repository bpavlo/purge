# Worktree: CI & Release

**Phase:** 3 (parallel with integration, after core is merged)
**Branch:** `worktree/ci-release`
**Dependencies:** Foundation (for buildable project)

---

## Goal

Set up CI pipeline, release automation, and repo polish. This is the "ship it" worktree.

---

## Tasks

### 1. GitHub Actions CI (`.github/workflows/ci.yml`)

- [ ] Trigger on: push to main, pull requests
- [ ] Matrix: Go 1.22+ on ubuntu-latest
- [ ] Steps:
  1. Checkout
  2. Setup Go
  3. Cache Go modules
  4. `go vet ./...`
  5. `golangci-lint run`
  6. `go test ./...`
  7. `go build -o purge .`

### 2. golangci-lint Config (`.golangci.yml`)

- [ ] Enable useful linters: `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosimple`
- [ ] Disable noisy ones for now
- [ ] Set timeout to 5 minutes

### 3. GoReleaser (`.goreleaser.yaml`)

- [ ] Build targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- [ ] Binary name: `purge`
- [ ] Archive format: tar.gz (linux/mac), zip (windows)
- [ ] Generate checksums
- [ ] Changelog from git commits

### 4. Release Workflow (`.github/workflows/release.yml`)

- [ ] Trigger on: tag push (`v*`)
- [ ] Steps:
  1. Checkout
  2. Setup Go
  3. Run GoReleaser
  4. Upload artifacts to GitHub Release

### 5. Nix Flake (`flake.nix`)

- [ ] `nix build` produces the binary
- [ ] `nix run github:user/purge` works
- [ ] Use `buildGoModule`
- [ ] Include `vendorHash`

### 6. Repo Files

- [ ] `LICENSE` — MIT license text
- [ ] `README.md`:
  - Project description and motivation
  - Install methods (go install, binary download, nix)
  - Quick start / usage examples
  - Supported platforms (Discord, Telegram)
  - Security / legal disclaimers
  - Placeholder for demo GIF/asciicast
- [ ] `CONTRIBUTING.md` — basic contribution guide
- [ ] `.github/ISSUE_TEMPLATE/bug_report.md`
- [ ] `.github/ISSUE_TEMPLATE/feature_request.md`
- [ ] `.github/dependabot.yml` — weekly Go module updates

---

## Acceptance Criteria

- [ ] CI passes on a clean PR
- [ ] `goreleaser check` passes
- [ ] `nix build` produces working binary (if Nix available)
- [ ] README has install instructions and usage examples
- [ ] Tagged release produces binaries for all target platforms

---

## Notes

- This can be worked on as soon as the project compiles (`go build` succeeds).
- README should reflect actual implemented features, not aspirational ones.
- Don't over-polish — this is MVP release infrastructure.
