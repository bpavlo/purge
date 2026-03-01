package filter

import (
	"testing"
	"time"

	"github.com/pavlo/purge/internal/types"
)

func makeMsg(content string, ts time.Time) *types.Message {
	return &types.Message{
		ID:        "1",
		Content:   content,
		Timestamp: ts,
	}
}

func TestMatchNoFilters(t *testing.T) {
	msg := makeMsg("hello", time.Now())
	if !Match(msg, Options{}) {
		t.Error("expected match with no filters")
	}
}

func TestMatchAfter(t *testing.T) {
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := Options{After: cutoff}

	before := makeMsg("old", cutoff.Add(-time.Hour))
	if Match(before, opts) {
		t.Error("should not match message before cutoff")
	}

	exact := makeMsg("exact", cutoff)
	if !Match(exact, opts) {
		t.Error("should match message at exact cutoff (inclusive)")
	}

	after := makeMsg("new", cutoff.Add(time.Hour))
	if !Match(after, opts) {
		t.Error("should match message after cutoff")
	}
}

func TestMatchBefore(t *testing.T) {
	cutoff := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	opts := Options{Before: cutoff}

	before := makeMsg("old", cutoff.Add(-time.Hour))
	if !Match(before, opts) {
		t.Error("should match message before cutoff")
	}

	exact := makeMsg("exact", cutoff)
	if !Match(exact, opts) {
		t.Error("should match message at exact cutoff (inclusive)")
	}

	after := makeMsg("new", cutoff.Add(time.Hour))
	if Match(after, opts) {
		t.Error("should not match message after cutoff")
	}
}

func TestMatchKeywordCaseInsensitive(t *testing.T) {
	msg := makeMsg("Hello World FOO", time.Now())

	if !Match(msg, Options{Keyword: "hello"}) {
		t.Error("keyword should be case-insensitive")
	}
	if !Match(msg, Options{Keyword: "WORLD"}) {
		t.Error("keyword should be case-insensitive")
	}
	if Match(msg, Options{Keyword: "bar"}) {
		t.Error("should not match absent keyword")
	}
}

func TestMatchHasAttachment(t *testing.T) {
	msg := makeMsg("text", time.Now())
	if Match(msg, Options{HasAttachment: true}) {
		t.Error("should not match message without attachment")
	}

	msg.HasAttachment = true
	if !Match(msg, Options{HasAttachment: true}) {
		t.Error("should match message with attachment")
	}
}

func TestMatchHasLink(t *testing.T) {
	msg := makeMsg("text", time.Now())
	if Match(msg, Options{HasLink: true}) {
		t.Error("should not match message without link")
	}

	msg.HasLink = true
	if !Match(msg, Options{HasLink: true}) {
		t.Error("should match message with link")
	}
}

func TestMatchMinLength(t *testing.T) {
	short := makeMsg("hi", time.Now())
	if Match(short, Options{MinLength: 10}) {
		t.Error("should not match short message")
	}

	long := makeMsg("this is a long message", time.Now())
	if !Match(long, Options{MinLength: 10}) {
		t.Error("should match long message")
	}
}

func TestMatchExcludePinned(t *testing.T) {
	msg := makeMsg("pinned", time.Now())
	msg.IsPinned = true

	if Match(msg, Options{ExcludePinned: true}) {
		t.Error("should not match pinned message when excluding pinned")
	}

	msg.IsPinned = false
	if !Match(msg, Options{ExcludePinned: true}) {
		t.Error("should match non-pinned message")
	}
}

func TestMatchCombinedFilters(t *testing.T) {
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := Options{
		After:         cutoff,
		Keyword:       "hello",
		HasAttachment: true,
		MinLength:     5,
		ExcludePinned: true,
	}

	msg := &types.Message{
		ID:            "1",
		Content:       "hello world attachment",
		Timestamp:     cutoff.Add(time.Hour),
		HasAttachment: true,
		IsPinned:      false,
	}

	if !Match(msg, opts) {
		t.Error("should match message satisfying all conditions")
	}

	// Fail one condition: keyword
	msg.Content = "goodbye world attachment"
	if Match(msg, opts) {
		t.Error("should not match when keyword doesn't match")
	}
}
