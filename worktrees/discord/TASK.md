# Worktree: Discord

**Phase:** 2 (after foundation + shared-core are merged)
**Branch:** `worktree/discord`
**Dependencies:** Foundation, Shared Core

---

## Goal

Implement the full Discord integration: auth, scan, delete, and archive using Discord's undocumented user API. This is raw HTTP — no SDK.

---

## Reference

- Discord user API is **undocumented**. Base URL: `https://discord.com/api/v9`
- Auth via `Authorization` header (user token from browser DevTools)
- Rate limits via `429` status + `Retry-After` header
- No bulk delete for user tokens — one `DELETE` per message
- Search via `GET /guilds/{id}/messages/search?author_id={self}`

---

## Tasks

### 1. Discord Client (`internal/discord/client.go`)

- [ ] `Client` struct:
  - `token string`
  - `httpClient *http.Client`
  - `rateLimiter *ratelimit.RateLimiter`
  - `baseURL string` (default `https://discord.com/api/v9`)
- [ ] `NewClient(token string, rl *ratelimit.RateLimiter) *Client`
- [ ] `doRequest(method, path string, body io.Reader) (*http.Response, error)`:
  - Set `Authorization` header (no "Bot " prefix — this is a user token)
  - Set `Content-Type: application/json`
  - Handle 429 responses: parse `Retry-After`, call rate limiter, retry
  - Handle 401/403: return typed auth error
  - Respect context cancellation
- [ ] `ValidateToken() (*User, error)` — `GET /users/@me`

### 2. Discord Types (`internal/discord/types.go`)

- [ ] `User` struct (id, username, discriminator)
- [ ] `Guild` struct (id, name)
- [ ] `Channel` struct (id, name, type, guild_id)
- [ ] `Message` struct (id, content, timestamp, author, attachments, pinned, type)
- [ ] `Attachment` struct (id, filename, url, size, content_type)
- [ ] `SearchResponse` struct (messages, total_results)
- [ ] Conversion function: `func (m *Message) ToCommon() *types.Message`

### 3. Discord Operations (`internal/discord/messages.go`)

#### List Guilds
- [ ] `GetGuilds() ([]Guild, error)` — `GET /users/@me/guilds`
- [ ] Find guild by name or ID

#### List Channels
- [ ] `GetChannels(guildID string) ([]Channel, error)` — `GET /guilds/{id}/channels`
- [ ] `GetDMChannels() ([]Channel, error)` — `GET /users/@me/channels`

#### Search Messages
- [ ] `SearchGuildMessages(guildID string, opts SearchOptions) (*SearchResponse, error)`
  - `GET /guilds/{id}/messages/search?author_id={self}&offset={n}`
  - Pagination via `offset` parameter (25 results per page)
  - Support `content`, `min_id`/`max_id` for date filtering
- [ ] `SearchChannelMessages(channelID string, opts SearchOptions) (*SearchResponse, error)`
  - `GET /channels/{id}/messages/search?author_id={self}`
- [ ] `GetChannelMessages(channelID string, before string, limit int) ([]Message, error)`
  - `GET /channels/{id}/messages?before={id}&limit={limit}`
  - For DMs where search isn't available

#### Delete Message
- [ ] `DeleteMessage(channelID, messageID string) error`
  - `DELETE /channels/{id}/messages/{id}`
  - Rate limit: ~1 req/sec (configurable)
  - Handle 404 (already deleted) — skip silently
  - Handle 403 (no permission) — return typed error

### 4. Auth Flow (`cmd/auth.go` discord portion)

- [ ] Print instructions:
  ```
  To get your Discord token:
  1. Open Discord in your browser (not the desktop app)
  2. Press F12 to open DevTools
  3. Go to the Network tab
  4. Send a message or refresh
  5. Find a request to discord.com/api
  6. Copy the 'Authorization' header value
  ```
- [ ] Prompt for token input (masked/hidden)
- [ ] Validate via `GET /api/v9/users/@me`
- [ ] Store in `~/.config/purge/discord_token` with `0600` permissions
- [ ] Print success: "Authenticated as {username}#{discriminator}"
- [ ] Warn about token sensitivity

### 5. Scan Wiring (`cmd/scan.go` discord portion)

- [ ] Load token from stored file
- [ ] Resolve `--server` to guild ID (by name or direct ID)
- [ ] Resolve `--channel` to channel ID
- [ ] If `--dms`: list DM channels
- [ ] If `--all-chats`: list all guilds + all channels
- [ ] For each target channel/guild: search messages with author filter
- [ ] Apply shared filters (`internal/filter`)
- [ ] Display results using `internal/ui` summary table

### 6. Delete Wiring (`cmd/delete.go` discord portion)

- [ ] Scan first (reuse scan logic to get message list)
- [ ] If `--dry-run`: show what would be deleted, exit
- [ ] If `--archive`: archive messages before deleting
- [ ] Show confirmation prompt (unless `--yes`)
- [ ] Delete messages one by one with rate limiting
- [ ] Update progress bar
- [ ] Save checkpoint on SIGINT
- [ ] Resume from checkpoint if one exists
- [ ] Print summary: deleted / failed / skipped counts

### 7. Archive Wiring (`cmd/archive.go` discord portion)

- [ ] Scan messages (reuse scan logic)
- [ ] Convert to common types
- [ ] Write to JSON using `internal/archive`
- [ ] Print output file paths

---

## Key API Endpoints

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Validate token | GET | `/api/v9/users/@me` |
| List guilds | GET | `/api/v9/users/@me/guilds` |
| List guild channels | GET | `/api/v9/guilds/{id}/channels` |
| List DM channels | GET | `/api/v9/users/@me/channels` |
| Search guild messages | GET | `/api/v9/guilds/{id}/messages/search?author_id={id}` |
| Get channel messages | GET | `/api/v9/channels/{id}/messages?before={id}&limit=100` |
| Delete message | DELETE | `/api/v9/channels/{id}/messages/{id}` |
| Get pinned messages | GET | `/api/v9/channels/{id}/pins` |

---

## Acceptance Criteria

- [ ] `purge auth discord` — full auth flow works, token stored securely
- [ ] `purge scan discord --server "X"` — lists messages with counts per channel
- [ ] `purge scan discord --dms` — scans all DM channels
- [ ] `purge delete discord --server "X" --dry-run` — shows what would be deleted
- [ ] `purge delete discord --server "X" --yes` — actually deletes messages
- [ ] `purge delete discord --server "X" --archive` — archives then deletes
- [ ] `purge archive discord --dms -o ~/backup/` — exports DMs to JSON
- [ ] Rate limiting works (no 429 spam)
- [ ] Ctrl+C saves checkpoint, next run offers resume
- [ ] All filters work: `--after`, `--before`, `--keyword`, `--has-attachment`, `--exclude-pinned`

---

## Notes

- Discord user API is against their ToS. Document this clearly in auth flow output.
- The search endpoint returns messages grouped by channel with surrounding context — parse carefully.
- Some DMs might not appear in search — fall back to paginating channel messages.
- User tokens do NOT use the "Bot " prefix in the Authorization header.
- Rate limits differ per route — the delete endpoint is the most restricted (~1/sec).
