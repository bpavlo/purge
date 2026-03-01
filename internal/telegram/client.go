// Package telegram implements the Telegram MTProto client for message scanning and deletion.
package telegram

import (
	"context"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"github.com/bpavlo/purge/internal/ratelimit"
)

// Client wraps the gotd Telegram client with session management and rate limiting.
type Client struct {
	client      *telegram.Client
	api         *tg.Client
	rateLimiter *ratelimit.RateLimiter
	self        *tg.User
	waiter      *floodwait.Waiter
}

// NewClient creates a new Telegram client with file-based session storage and floodwait middleware.
func NewClient(apiID int, apiHash string, sessionPath string, rl *ratelimit.RateLimiter) *Client {
	waiter := floodwait.NewWaiter()

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{
			Path: sessionPath,
		},
		Middlewares: []telegram.Middleware{
			waiter,
		},
	})

	if rl == nil {
		rl = ratelimit.New(ratelimit.DefaultConfig())
	}

	return &Client{
		client:      client,
		rateLimiter: rl,
		waiter:      waiter,
	}
}

// Run executes f within the gotd client lifecycle. All API calls must happen inside f.
// The api field is populated before f is called.
func (c *Client) Run(ctx context.Context, f func(ctx context.Context) error) error {
	return c.waiter.Run(ctx, func(ctx context.Context) error {
		return c.client.Run(ctx, func(ctx context.Context) error {
			c.api = c.client.API()
			return f(ctx)
		})
	})
}

// API returns the raw tg.Client for making API calls. Only valid inside Run().
func (c *Client) API() *tg.Client {
	return c.api
}

// Auth returns the auth client for running authentication flows.
func (c *Client) Auth() *auth.Client {
	return c.client.Auth()
}

// RateLimiter returns the rate limiter instance.
func (c *Client) RateLimiter() *ratelimit.RateLimiter {
	return c.rateLimiter
}

// GetSelf fetches and caches the currently authenticated user.
func (c *Client) GetSelf(ctx context.Context) (*tg.User, error) {
	if c.self != nil {
		return c.self, nil
	}

	result, err := c.api.UsersGetUsers(ctx, []tg.InputUserClass{&tg.InputUserSelf{}})
	if err != nil {
		return nil, err
	}

	for _, u := range result {
		if user, ok := u.(*tg.User); ok {
			c.self = user
			return user, nil
		}
	}

	return nil, ErrUserNotFound
}

// Self returns the cached current user. Returns nil if GetSelf has not been called.
func (c *Client) Self() *tg.User {
	return c.self
}
