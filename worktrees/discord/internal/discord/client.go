package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pavlo/purge/internal/ratelimit"
)

const (
	// DefaultBaseURL is the Discord API v9 base URL.
	DefaultBaseURL = "https://discord.com/api/v9"

	// maxRetries is the maximum number of retries for rate-limited requests.
	maxRetries = 5
)

// ErrAuth represents an authentication or authorization failure.
type ErrAuth struct {
	StatusCode int
	Message    string
}

func (e *ErrAuth) Error() string {
	return fmt.Sprintf("discord auth error (HTTP %d): %s", e.StatusCode, e.Message)
}

// ErrForbidden represents a 403 Forbidden response.
type ErrForbidden struct {
	Message string
}

func (e *ErrForbidden) Error() string {
	return fmt.Sprintf("discord forbidden: %s", e.Message)
}

// ErrRateLimit represents a rate limit that has been exhausted after max retries.
type ErrRateLimit struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimit) Error() string {
	return fmt.Sprintf("discord rate limit exceeded, retry after %s", e.RetryAfter)
}

// ErrNotFound represents a 404 Not Found response.
type ErrNotFound struct {
	Message string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("discord not found: %s", e.Message)
}

// Client is a Discord API client that uses raw HTTP with user token authentication.
type Client struct {
	token       string
	httpClient  *http.Client
	rateLimiter *ratelimit.RateLimiter
	baseURL     string
	selfUser    *User // cached after ValidateToken
}

// NewClient creates a new Discord API client.
func NewClient(token string, rl *ratelimit.RateLimiter) *Client {
	return &Client{
		token:       token,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: rl,
		baseURL:     DefaultBaseURL,
	}
}

// SetBaseURL overrides the default API base URL (useful for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// SelfUser returns the cached authenticated user, or nil if not yet validated.
func (c *Client) SelfUser() *User {
	return c.selfUser
}

// ValidateToken validates the Discord token by calling GET /users/@me.
// On success, it caches the user and returns it.
func (c *Client) ValidateToken(ctx context.Context) (*User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}
	defer resp.Body.Close()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding user response: %w", err)
	}

	c.selfUser = &user
	return &user, nil
}

// doRequest performs an HTTP request with rate limiting, retry on 429, and
// typed error handling for 401/403/404.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Wait for rate limiter before each attempt
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter wait: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusOK, http.StatusNoContent:
			return resp, nil

		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp)
			resp.Body.Close()

			c.rateLimiter.HandleRateLimit(retryAfter)

			if attempt == maxRetries {
				return nil, &ErrRateLimit{RetryAfter: retryAfter}
			}

			// Wait for the retry-after duration
			select {
			case <-time.After(retryAfter):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}

		case http.StatusUnauthorized:
			resp.Body.Close()
			return nil, &ErrAuth{
				StatusCode: http.StatusUnauthorized,
				Message:    "invalid or expired token",
			}

		case http.StatusForbidden:
			resp.Body.Close()
			return nil, &ErrForbidden{
				Message: fmt.Sprintf("%s %s: access denied", method, path),
			}

		case http.StatusNotFound:
			resp.Body.Close()
			return nil, &ErrNotFound{
				Message: fmt.Sprintf("%s %s: not found", method, path),
			}

		default:
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status %d for %s %s: %s",
				resp.StatusCode, method, path, string(bodyBytes))
		}
	}

	return nil, fmt.Errorf("max retries exceeded for %s %s", method, path)
}

// parseRetryAfter parses the Retry-After header from a 429 response.
// Discord sends Retry-After as a float (seconds). Falls back to 5s if missing.
func parseRetryAfter(resp *http.Response) time.Duration {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		// Also check JSON body for retry_after field
		var rateLimitBody struct {
			RetryAfter float64 `json:"retry_after"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rateLimitBody); err == nil && rateLimitBody.RetryAfter > 0 {
			return time.Duration(rateLimitBody.RetryAfter * float64(time.Second))
		}
		return 5 * time.Second
	}

	// Try parsing as float first (Discord's format)
	if seconds, err := strconv.ParseFloat(header, 64); err == nil {
		return time.Duration(seconds * float64(time.Second))
	}

	// Try parsing as integer seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	return 5 * time.Second
}
