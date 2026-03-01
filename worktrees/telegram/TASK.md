# Worktree: Telegram

**Phase:** 2 (after foundation + shared-core are merged, parallel with Discord)
**Branch:** `worktree/telegram`
**Dependencies:** Foundation, Shared Core

---

## Goal

Implement full Telegram integration using `gotd/td` (MTProto user API): auth, scan, delete (with batch support), and archive.

---

## Reference

- Library: `github.com/gotd/td` (v0.140.0, stable, MIT)
- Auth: `td/telegram/auth.Flow` handles phone + OTP + 2FA
- Session: pluggable file-based storage
- Batch delete: `messages.deleteMessages` — up to 100 IDs per call
- FLOOD_WAIT: use `github.com/gotd/contrib/middleware/floodwait`
- Pagination: `telegram/query/messages` package

---

## Tasks

### 1. Telegram Client (`internal/telegram/client.go`)

- [ ] `Client` struct:
  - `client *telegram.Client` (gotd client)
  - `api *tg.Client` (raw API accessor)
  - `rateLimiter *ratelimit.RateLimiter`
  - `self *tg.User` (current user, populated after auth)
- [ ] `NewClient(apiID int, apiHash string, sessionPath string) (*Client, error)`
  - Create `telegram.NewClient(apiID, apiHash, telegram.Options{...})`
  - Configure session storage to `sessionPath`
  - Add floodwait middleware from `gotd/contrib`
- [ ] `Run(ctx context.Context, f func(ctx context.Context) error) error`
  - Wraps `client.Run()` — gotd requires all API calls within this callback
- [ ] `GetSelf() (*tg.User, error)` — fetch current authenticated user

### 2. Telegram Types (`internal/telegram/types.go`)

- [ ] `Chat` struct (id, title, type — private/group/supergroup/channel)
- [ ] Conversion: `func MessageToCommon(msg *tg.Message, chat Chat) *types.Message`
  - Map `tg.Message` fields to `types.Message`
  - Extract content, timestamp, attachments, media info
  - Determine message type (text, photo, document, etc.)
- [ ] Helper to determine if user can delete in a given chat type

### 3. Auth Flow (`internal/telegram/auth.go`)

- [ ] Use `auth.NewFlow()` with:
  - `auth.Constant(phone, password, codeAuthenticator)`
  - `codeAuthenticator` — prompt user for OTP code from terminal
- [ ] Flow:
  1. Prompt for API ID and API Hash (or read from config/env)
  2. Store API ID/Hash in `~/.config/purge/telegram_config` (0600 perms)
  3. Prompt for phone number
  4. `auth.Flow` handles sending code + prompting OTP
  5. Handle 2FA password prompt if needed
  6. Session auto-saved by gotd session storage
  7. Validate by fetching `users.getFullUser(self)`
- [ ] Print success: "Authenticated as {first_name} {last_name} (@{username})"
- [ ] On subsequent runs: check if session is still valid, skip auth if so

### 4. Telegram Operations (`internal/telegram/messages.go`)

#### List Chats/Dialogs
- [ ] `GetDialogs() ([]Chat, error)`
  - Use `messages.getDialogs` or gotd's dialog iteration helpers
  - Return list of all accessible chats with metadata
  - Support filtering by chat type (private, group, channel)

#### Search/Fetch Messages
- [ ] `GetMessages(chatID int64, opts SearchOptions) ([]tg.Message, error)`
  - Use `messages.search` with `from_id` set to self
  - Or iterate history with `messages.getHistory` + filter by author
  - Support pagination via gotd's `query/messages` helpers
  - Handle `min_date`/`max_date` for date filtering
  - Handle keyword search via `q` parameter in `messages.search`
- [ ] `GetMessageCount(chatID int64, opts SearchOptions) (int, error)`
  - Quick count without fetching all messages (use search with limit=0 if API supports)

#### Delete Messages
- [ ] `DeleteMessages(chatID int64, messageIDs []int, revoke bool) (int, error)`
  - Use `messages.deleteMessages` for private chats
  - Use `channels.deleteMessages` for channels/supergroups
  - Batch: up to 100 IDs per call
  - `revoke: true` to delete for both sides in private chats
  - Return count of actually deleted messages
- [ ] `BatchDelete(chatID int64, allIDs []int, revoke bool, progress func(deleted int)) error`
  - Split into chunks of 100
  - Rate limit between batches
  - Call progress callback after each batch
  - Save checkpoint between batches

### 5. Scan Wiring (`cmd/scan.go` telegram portion)

- [ ] Load session / check auth status
- [ ] List dialogs, filter by `--chat` name/ID
- [ ] If `--dms`: filter to private chats only
- [ ] If `--all-chats`: include everything
- [ ] For each target chat: count/fetch messages from self
- [ ] Apply shared filters (`internal/filter`)
- [ ] Display results using `internal/ui` summary table

### 6. Delete Wiring (`cmd/delete.go` telegram portion)

- [ ] Scan first to get message IDs
- [ ] If `--dry-run`: show what would be deleted, exit
- [ ] If `--archive`: archive messages first
- [ ] Confirmation prompt (unless `--yes`)
- [ ] Batch delete with progress bar
- [ ] Rate limit between batches (default 500ms)
- [ ] Checkpoint save on SIGINT
- [ ] Resume from checkpoint
- [ ] Print summary

### 7. Archive Wiring (`cmd/archive.go` telegram portion)

- [ ] Fetch messages (reuse scan logic)
- [ ] Convert to common types
- [ ] Write JSON using `internal/archive`

---

## Key gotd/td Patterns

```go
// Client setup
client := telegram.NewClient(apiID, apiHash, telegram.Options{
    SessionStorage: &session.FileStorage{Path: sessionPath},
    Middlewares: []telegram.Middleware{
        floodwait.NewSimpleWaiter(),
    },
})

// All API calls must happen inside Run()
client.Run(ctx, func(ctx context.Context) error {
    api := client.API()

    // Auth
    flow := auth.NewFlow(auth.Constant(phone, pwd, codePrompt), auth.SendCodeOptions{})
    if err := flow.Run(ctx, client.Auth()); err != nil { ... }

    // Search messages
    result, err := api.MessagesSearch(ctx, &tg.MessagesSearchRequest{
        Peer:     peer,
        Q:        keyword,
        FromID:   &tg.InputPeerUser{UserID: selfID},
        MinDate:  int(afterDate.Unix()),
        MaxDate:  int(beforeDate.Unix()),
        Limit:    100,
        OffsetID: lastSeenID,
    })

    // Batch delete (private chats)
    affected, err := api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
        Revoke: true,
        ID:     messageIDs, // up to 100
    })

    // Batch delete (channels/supergroups)
    affected, err := api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
        Channel: &tg.InputChannel{ChannelID: channelID, AccessHash: hash},
        ID:      messageIDs,
    })
})
```

---

## Acceptance Criteria

- [ ] `purge auth telegram` — full auth flow (API ID + phone + OTP + optional 2FA)
- [ ] Session persists — re-running doesn't re-auth
- [ ] `purge scan telegram --chat "X"` — shows message counts
- [ ] `purge scan telegram --dms` — scans all private chats
- [ ] `purge delete telegram --chat "X" --dry-run` — preview mode works
- [ ] `purge delete telegram --chat "X" --yes` — batch deletes messages
- [ ] Batch deletion uses 100-ID chunks (not one-by-one)
- [ ] Rate limiting between batches works
- [ ] `purge archive telegram --chat "X" -o ~/backup/` — exports to JSON
- [ ] All filters work
- [ ] Ctrl+C saves checkpoint

---

## Notes

- **48-hour rule**: For non-admin users in some chats, messages older than 48h may fail to delete. Handle this gracefully — skip and count as "failed", don't crash.
- **Peer resolution**: Converting chat names to `InputPeer` types requires dialog listing. Cache resolved peers.
- **Channel vs Chat**: `messages.deleteMessages` works for private chats and basic groups. For supergroups/channels, use `channels.deleteMessages`. Check chat type before calling.
- **gotd client lifecycle**: All API calls MUST happen inside `client.Run()`. Structure the code so the CLI commands call into a function that runs within this scope.
- The floodwait middleware from `gotd/contrib` handles most FLOOD_WAIT automatically, but add our own rate limiter on top for politeness.
