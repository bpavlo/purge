package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// GetGuilds returns all guilds (servers) the user is a member of.
func (c *Client) GetGuilds(ctx context.Context) ([]Guild, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users/@me/guilds", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching guilds: %w", err)
	}
	defer resp.Body.Close()

	var guilds []Guild
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return nil, fmt.Errorf("decoding guilds: %w", err)
	}

	return guilds, nil
}

// FindGuild looks up a guild by name or ID from the user's guild list.
func (c *Client) FindGuild(ctx context.Context, nameOrID string) (*Guild, error) {
	guilds, err := c.GetGuilds(ctx)
	if err != nil {
		return nil, err
	}

	for _, g := range guilds {
		if g.ID == nameOrID || g.Name == nameOrID {
			return &g, nil
		}
	}

	return nil, fmt.Errorf("guild not found: %s", nameOrID)
}

// GetChannels returns all channels in a guild.
func (c *Client) GetChannels(ctx context.Context, guildID string) ([]Channel, error) {
	path := fmt.Sprintf("/guilds/%s/channels", guildID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching channels for guild %s: %w", guildID, err)
	}
	defer resp.Body.Close()

	var channels []Channel
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		return nil, fmt.Errorf("decoding channels: %w", err)
	}

	return channels, nil
}

// GetTextChannels returns only text-based channels from a guild (text, news, forum).
func (c *Client) GetTextChannels(ctx context.Context, guildID string) ([]Channel, error) {
	channels, err := c.GetChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}

	var textChannels []Channel
	for _, ch := range channels {
		switch ch.Type {
		case ChannelTypeGuildText, ChannelTypeGuildNews, ChannelTypeGuildForum:
			textChannels = append(textChannels, ch)
		}
	}

	return textChannels, nil
}

// GetDMChannels returns all DM and group DM channels.
func (c *Client) GetDMChannels(ctx context.Context) ([]Channel, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users/@me/channels", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching DM channels: %w", err)
	}
	defer resp.Body.Close()

	var channels []Channel
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		return nil, fmt.Errorf("decoding DM channels: %w", err)
	}

	return channels, nil
}

// SearchGuildMessages searches for messages by the authenticated user in a guild.
// Discord returns 25 results per page; use opts.Offset for pagination.
func (c *Client) SearchGuildMessages(ctx context.Context, guildID string, opts SearchOptions) (*SearchResponse, error) {
	if c.selfUser == nil {
		return nil, fmt.Errorf("must call ValidateToken before searching (self user not cached)")
	}

	authorID := opts.AuthorID
	if authorID == "" {
		authorID = c.selfUser.ID
	}

	params := url.Values{}
	params.Set("author_id", authorID)

	if opts.Content != "" {
		params.Set("content", opts.Content)
	}
	if opts.MinID != "" {
		params.Set("min_id", opts.MinID)
	}
	if opts.MaxID != "" {
		params.Set("max_id", opts.MaxID)
	}
	if opts.Offset > 0 {
		params.Set("offset", strconv.Itoa(opts.Offset))
	}

	path := fmt.Sprintf("/guilds/%s/messages/search?%s", guildID, params.Encode())
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("searching guild messages: %w", err)
	}
	defer resp.Body.Close()

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return &searchResp, nil
}

// SearchChannelMessages searches for messages by the authenticated user in a specific channel.
func (c *Client) SearchChannelMessages(ctx context.Context, channelID string, opts SearchOptions) (*SearchResponse, error) {
	if c.selfUser == nil {
		return nil, fmt.Errorf("must call ValidateToken before searching (self user not cached)")
	}

	authorID := opts.AuthorID
	if authorID == "" {
		authorID = c.selfUser.ID
	}

	params := url.Values{}
	params.Set("author_id", authorID)

	if opts.Content != "" {
		params.Set("content", opts.Content)
	}
	if opts.MinID != "" {
		params.Set("min_id", opts.MinID)
	}
	if opts.MaxID != "" {
		params.Set("max_id", opts.MaxID)
	}
	if opts.Offset > 0 {
		params.Set("offset", strconv.Itoa(opts.Offset))
	}

	path := fmt.Sprintf("/channels/%s/messages/search?%s", channelID, params.Encode())
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("searching channel messages: %w", err)
	}
	defer resp.Body.Close()

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return &searchResp, nil
}

// GetChannelMessages fetches messages from a channel using pagination.
// This is used for DMs where the search endpoint may not be available.
// Use before="" to start from the latest message. limit is capped at 100 by Discord.
func (c *Client) GetChannelMessages(ctx context.Context, channelID string, before string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	if before != "" {
		params.Set("before", before)
	}

	path := fmt.Sprintf("/channels/%s/messages?%s", channelID, params.Encode())
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching channel messages: %w", err)
	}
	defer resp.Body.Close()

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("decoding channel messages: %w", err)
	}

	return messages, nil
}

// DeleteMessage deletes a single message from a channel.
// Returns (true, nil) if the message was already deleted (404).
// Returns (false, nil) on successful deletion.
// Returns (false, *ErrForbidden) if the user lacks permission.
func (c *Client) DeleteMessage(ctx context.Context, channelID, messageID string) (alreadyDeleted bool, err error) {
	path := fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID)

	// Use a dedicated route key for delete rate limiting
	if err := c.rateLimiter.WaitRoute(ctx, "DELETE:message"); err != nil {
		return false, fmt.Errorf("rate limiter wait: %w", err)
	}

	_, err = c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		// 404 means already deleted
		var notFoundErr *ErrNotFound
		if errors.As(err, &notFoundErr) {
			return true, nil
		}
		return false, fmt.Errorf("deleting message %s in channel %s: %w", messageID, channelID, err)
	}

	return false, nil
}
