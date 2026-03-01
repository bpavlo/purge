# Worktree: Foundation

**Phase:** 1 (start first, before all others)
**Branch:** `worktree/foundation`
**Dependencies:** None

---

## Goal

Set up the Go project scaffolding, CLI framework (Cobra + Viper), and all command stubs. This worktree produces a compilable binary with `purge auth|scan|delete|archive` subcommands that accept all flags but print "not implemented" for platform-specific logic.

---

## Tasks

### 1. Project Init
- [ ] `go mod init github.com/pavlo/purge` (adjust module path as needed)
- [ ] Create directory structure:
  ```
  purge/
  ├── cmd/
  │   ├── root.go
  │   ├── auth.go
  │   ├── scan.go
  │   ├── delete.go
  │   └── archive.go
  ├── internal/
  │   ├── discord/
  │   ├── telegram/
  │   ├── filter/
  │   ├── archive/
  │   ├── ratelimit/
  │   └── ui/
  ├── configs/
  │   └── purge.example.yaml
  ├── main.go
  └── Makefile
  ```
- [ ] Add `.gitignore` for Go projects

### 2. Main Entrypoint
- [ ] `main.go` — calls `cmd.Execute()`
- [ ] Keep it minimal, just the entrypoint

### 3. Root Command (`cmd/root.go`)
- [ ] Initialize Cobra root command with app description
- [ ] Global persistent flags:
  - `-v, --verbose` — verbose output
  - `-q, --quiet` — quiet mode (errors + summary only)
  - `--json` — machine-readable JSON output
  - `--config` — path to config file (default `~/.config/purge/purge.yaml`)
  - `--log-level` — debug|info|warn|error
- [ ] Viper config loading:
  - Config file: `~/.config/purge/purge.yaml`
  - Env var prefix: `PURGE_`
  - Bind all persistent flags to viper
- [ ] Exit code constants (0=success, 1=error, 2=auth failure, 3=partial failure, 130=interrupted)

### 4. Auth Command (`cmd/auth.go`)
- [ ] `purge auth discord` — subcommand stub
- [ ] `purge auth telegram` — subcommand stub
- [ ] Both print instructions and accept platform argument
- [ ] No actual auth logic (that's in discord/telegram worktrees)

### 5. Scan Command (`cmd/scan.go`)
- [ ] `purge scan discord|telegram`
- [ ] Flags (all stubbed, parsed but not wired):
  - `--server` (Discord server name/ID)
  - `--channel` (Discord channel name/ID)
  - `--chat` (Telegram chat name/ID)
  - `--dms` (target all DMs/private chats)
  - `--all-chats` (target everything)
  - `--after` (date, inclusive)
  - `--before` (date, inclusive)
  - `--keyword` (case-insensitive text match)
  - `--has-attachment`
  - `--has-link`
  - `--min-length` (int)
  - `--exclude-pinned`

### 6. Delete Command (`cmd/delete.go`)
- [ ] `purge delete discord|telegram`
- [ ] Same filter flags as scan (reuse flag definitions)
- [ ] Additional flags:
  - `--yes` — skip confirmation prompt
  - `--dry-run` — preview only, don't delete
  - `--archive` — archive before deleting

### 7. Archive Command (`cmd/archive.go`)
- [ ] `purge archive discord|telegram`
- [ ] Same filter flags as scan
- [ ] Additional flags:
  - `--output` / `-o` — output directory (default from config `archive_dir`)

### 8. Example Config
- [ ] `configs/purge.example.yaml` with all documented options from SPEC.md section 8
- [ ] Comments explaining each option

### 9. Makefile
- [ ] `make build` — `go build -o purge .`
- [ ] `make run` — `go run .`
- [ ] `make test` — `go test ./...`
- [ ] `make lint` — `golangci-lint run` (if installed)
- [ ] `make clean`

---

## Acceptance Criteria

- [ ] `go build .` produces a `purge` binary
- [ ] `purge --help` shows all subcommands
- [ ] `purge scan --help` shows all filter flags
- [ ] `purge delete --help` shows filter flags + `--yes`, `--dry-run`, `--archive`
- [ ] `purge auth discord` prints a stub message
- [ ] Config file is loaded from `~/.config/purge/purge.yaml` if it exists
- [ ] Env vars with `PURGE_` prefix are recognized

---

## Notes

- Keep platform-specific logic out of this worktree. Use placeholder functions like `func runDiscordScan(opts ScanOptions) error { return fmt.Errorf("not implemented") }`
- Define a shared `FilterOptions` struct in `cmd/` that all commands can use — this will later be passed to `internal/filter/`
- The filter flags should be defined once and reused across scan/delete/archive commands (use a helper function to add them to any cobra.Command)
