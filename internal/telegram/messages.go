package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

const maxDeleteBatch = 100

// GetDialogs fetches all accessible dialogs and returns them as Chat values.
// It paginates through results until all dialogs are retrieved.
func (c *Client) GetDialogs(ctx context.Context) ([]Chat, error) {
	const pageLimit = 100

	var allChats []Chat
	offsetDate := 0
	offsetID := 0
	var offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}

	for {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}

		result, err := c.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			OffsetPeer: offsetPeer,
			Limit:      pageLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("get dialogs: %w", err)
		}

		chats, err := c.extractDialogChats(result)
		if err != nil {
			return nil, err
		}
		allChats = append(allChats, chats...)

		// MessagesDialogs means the server returned the complete list.
		if _, ok := result.(*tg.MessagesDialogs); ok {
			break
		}

		// No more results in this page — we've fetched everything.
		slice, ok := result.(*tg.MessagesDialogsSlice)
		if !ok || len(slice.Dialogs) == 0 {
			break
		}

		// All dialogs fetched.
		if len(allChats) >= slice.Count {
			break
		}

		// Advance the offset using the last message and dialog in the page.
		lastDialog := slice.Dialogs[len(slice.Dialogs)-1]
		d, ok := lastDialog.(*tg.Dialog)
		if !ok {
			break
		}

		// Find the date from the last message in the page.
		topMsgID := d.GetTopMessage()
		offsetID = topMsgID
		for _, m := range slice.Messages {
			if msg, ok := m.(*tg.Message); ok && msg.ID == topMsgID {
				offsetDate = msg.Date
				break
			}
		}

		offsetPeer = dialogPeerToInputPeer(d.Peer, slice.Users, slice.Chats)
	}

	return allChats, nil
}

// extractDialogChats builds Chat slices from dialog results.
func (c *Client) extractDialogChats(result tg.MessagesDialogsClass) ([]Chat, error) {
	var (
		dialogs  []tg.DialogClass
		users    []tg.UserClass
		chatList []tg.ChatClass
	)

	switch r := result.(type) {
	case *tg.MessagesDialogs:
		dialogs = r.Dialogs
		users = r.Users
		chatList = r.Chats
	case *tg.MessagesDialogsSlice:
		dialogs = r.Dialogs
		users = r.Users
		chatList = r.Chats
	default:
		return nil, fmt.Errorf("unexpected dialog result type: %T", result)
	}

	// Build lookup maps.
	userMap := make(map[int64]*tg.User, len(users))
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}

	chatMap := make(map[int64]*tg.Chat)
	channelMap := make(map[int64]*tg.Channel)
	for _, ch := range chatList {
		switch v := ch.(type) {
		case *tg.Chat:
			chatMap[v.ID] = v
		case *tg.Channel:
			channelMap[v.ID] = v
		}
	}

	var chats []Chat
	for _, d := range dialogs {
		dialog, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		chat, ok := ChatFromDialog(dialog.Peer, userMap, chatMap, channelMap)
		if ok {
			chats = append(chats, chat)
		}
	}

	return chats, nil
}

// dialogPeerToInputPeer converts a dialog's PeerClass to an InputPeerClass
// using the user/chat entity lists from the dialogs response.
func dialogPeerToInputPeer(peer tg.PeerClass, users []tg.UserClass, chats []tg.ChatClass) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerUser:
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		for _, ch := range chats {
			if channel, ok := ch.(*tg.Channel); ok && channel.ID == p.ChannelID {
				return &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash}
			}
		}
	}
	return &tg.InputPeerEmpty{}
}

// GetMessages searches for messages in a chat matching the given options.
// It paginates automatically and returns all matching tg.Message values.
func (c *Client) GetMessages(ctx context.Context, chat Chat, opts SearchOptions) ([]*tg.Message, error) {
	self, err := c.GetSelf(ctx)
	if err != nil {
		return nil, err
	}

	var allMessages []*tg.Message
	offsetID := opts.OffsetID
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	for {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}

		req := &tg.MessagesSearchRequest{
			Peer:     chat.InputPeer(),
			Q:        opts.Query,
			OffsetID: offsetID,
			Limit:    limit,
			Filter:   &tg.InputMessagesFilterEmpty{},
		}

		if opts.FromSelf {
			req.FromID = &tg.InputPeerUser{
				UserID:     self.ID,
				AccessHash: self.AccessHash,
			}
		}

		if !opts.MinDate.IsZero() {
			req.MinDate = int(opts.MinDate.Unix())
		}
		if !opts.MaxDate.IsZero() {
			req.MaxDate = int(opts.MaxDate.Unix())
		}

		result, err := c.api.MessagesSearch(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("search messages: %w", err)
		}

		messages := extractMessages(result)
		if len(messages) == 0 {
			break
		}

		allMessages = append(allMessages, messages...)

		// Use the last message ID for pagination offset.
		lastMsg := messages[len(messages)-1]
		offsetID = lastMsg.ID

		// If we got fewer than requested, we've reached the end.
		if len(messages) < limit {
			break
		}
	}

	return allMessages, nil
}

// GetMessageCount returns the count of messages matching the search options.
func (c *Client) GetMessageCount(ctx context.Context, chat Chat, opts SearchOptions) (int, error) {
	self, err := c.GetSelf(ctx)
	if err != nil {
		return 0, err
	}

	if err := c.rateLimiter.Wait(ctx); err != nil {
		return 0, err
	}

	req := &tg.MessagesSearchRequest{
		Peer:   chat.InputPeer(),
		Q:      opts.Query,
		Limit:  1, // We only need the count, not actual messages.
		Filter: &tg.InputMessagesFilterEmpty{},
	}

	if opts.FromSelf {
		req.FromID = &tg.InputPeerUser{
			UserID:     self.ID,
			AccessHash: self.AccessHash,
		}
	}

	if !opts.MinDate.IsZero() {
		req.MinDate = int(opts.MinDate.Unix())
	}
	if !opts.MaxDate.IsZero() {
		req.MaxDate = int(opts.MaxDate.Unix())
	}

	result, err := c.api.MessagesSearch(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("search message count: %w", err)
	}

	switch r := result.(type) {
	case *tg.MessagesMessages:
		return len(r.Messages), nil
	case *tg.MessagesMessagesSlice:
		return r.Count, nil
	case *tg.MessagesChannelMessages:
		return r.Count, nil
	default:
		return 0, fmt.Errorf("unexpected search result type: %T", result)
	}
}

// DeleteMessages deletes messages by ID in the given chat.
// It automatically uses the correct API method based on chat type (channel vs regular).
// Messages are batched into groups of up to 100 IDs per API call.
// Returns the total number of affected messages.
func (c *Client) DeleteMessages(ctx context.Context, chat Chat, messageIDs []int, revoke bool) (int, error) {
	if len(messageIDs) == 0 {
		return 0, nil
	}

	totalAffected := 0

	for i := 0; i < len(messageIDs); i += maxDeleteBatch {
		end := i + maxDeleteBatch
		if end > len(messageIDs) {
			end = len(messageIDs)
		}
		batch := messageIDs[i:end]

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return totalAffected, err
		}

		affected, err := c.deleteBatch(ctx, chat, batch, revoke)
		if err != nil {
			return totalAffected, fmt.Errorf("delete batch at offset %d: %w", i, err)
		}
		totalAffected += affected
	}

	return totalAffected, nil
}

// deleteBatch performs a single batch delete call (up to 100 IDs).
func (c *Client) deleteBatch(ctx context.Context, chat Chat, ids []int, revoke bool) (int, error) {
	if chat.IsChannel() {
		result, err := c.api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: chat.InputChannel(),
			ID:      ids,
		})
		if err != nil {
			return 0, err
		}
		return result.PtsCount, nil
	}

	result, err := c.api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		Revoke: revoke,
		ID:     ids,
	})
	if err != nil {
		return 0, err
	}
	return result.PtsCount, nil
}

// BatchDelete deletes all given message IDs in chunks, calling progress after each batch.
// It respects rate limiting between batches.
func (c *Client) BatchDelete(ctx context.Context, chat Chat, allIDs []int, revoke bool, progress func(deleted int)) error {
	totalDeleted := 0

	for i := 0; i < len(allIDs); i += maxDeleteBatch {
		end := i + maxDeleteBatch
		if end > len(allIDs) {
			end = len(allIDs)
		}
		batch := allIDs[i:end]

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit wait: %w", err)
		}

		affected, err := c.deleteBatch(ctx, chat, batch, revoke)
		if err != nil {
			return fmt.Errorf("batch delete at offset %d: %w", i, err)
		}

		totalDeleted += affected
		if progress != nil {
			progress(totalDeleted)
		}
	}

	return nil
}

// extractMessages pulls tg.Message values from various MessagesMessages result types.
func extractMessages(result tg.MessagesMessagesClass) []*tg.Message {
	var rawMessages []tg.MessageClass

	switch r := result.(type) {
	case *tg.MessagesMessages:
		rawMessages = r.Messages
	case *tg.MessagesMessagesSlice:
		rawMessages = r.Messages
	case *tg.MessagesChannelMessages:
		rawMessages = r.Messages
	default:
		return nil
	}

	var messages []*tg.Message
	for _, m := range rawMessages {
		if msg, ok := m.(*tg.Message); ok {
			messages = append(messages, msg)
		}
	}
	return messages
}
