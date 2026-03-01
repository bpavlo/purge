// Package filter provides message filtering based on CLI flag options.
package filter

import (
	"strings"
	"time"

	"github.com/pavlo/purge/internal/types"
)

// Options defines all available filter criteria. All conditions are combined with AND logic.
type Options struct {
	After         time.Time // Inclusive (>=)
	Before        time.Time // Inclusive (<=)
	Keyword       string    // Case-insensitive substring match
	HasAttachment bool
	HasLink       bool
	MinLength     int
	ExcludePinned bool
}

// Match returns true if the message satisfies all filter conditions.
func Match(msg *types.Message, opts Options) bool {
	if !opts.After.IsZero() && msg.Timestamp.Before(opts.After) {
		return false
	}

	if !opts.Before.IsZero() && msg.Timestamp.After(opts.Before) {
		return false
	}

	if opts.Keyword != "" {
		if !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(opts.Keyword)) {
			return false
		}
	}

	if opts.HasAttachment && !msg.HasAttachment {
		return false
	}

	if opts.HasLink && !msg.HasLink {
		return false
	}

	if opts.MinLength > 0 && len(msg.Content) < opts.MinLength {
		return false
	}

	if opts.ExcludePinned && msg.IsPinned {
		return false
	}

	return true
}
