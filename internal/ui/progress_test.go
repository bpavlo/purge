package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bpavlo/purge/internal/types"
)

func TestNewProgressBarNormal(t *testing.T) {
	pb := NewProgressBar(100, "Testing", ModeNormal)
	if pb == nil {
		t.Fatal("expected non-nil progress bar")
	}
	if pb.total != 100 {
		t.Errorf("expected total 100, got %d", pb.total)
	}
	if pb.mode != ModeNormal {
		t.Errorf("expected ModeNormal, got %d", pb.mode)
	}
}

func TestNewProgressBarQuiet(t *testing.T) {
	pb := NewProgressBar(50, "Quiet", ModeQuiet)
	if pb == nil {
		t.Fatal("expected non-nil progress bar")
	}
	// Should not panic on operations
	pb.Increment()
	pb.SetStatus("paused")
	pb.Finish()
}

func TestNewProgressBarJSON(t *testing.T) {
	pb := NewProgressBar(50, "JSON", ModeJSON)
	if pb == nil {
		t.Fatal("expected non-nil progress bar")
	}
	pb.Increment()
	pb.SetStatus("paused")
	pb.Finish()
}

func TestFormatSummaryTableEmpty(t *testing.T) {
	result := FormatSummaryTable(nil)
	if result != "No results found." {
		t.Errorf("expected 'No results found.', got %q", result)
	}
}

func TestFormatSummaryTable(t *testing.T) {
	results := []types.ScanResult{
		{
			Channel:      types.Channel{Name: "#general", ID: "123"},
			MessageCount: 342,
			FirstDate:    time.Date(2022, 3, 15, 0, 0, 0, 0, time.UTC),
			LastDate:     time.Date(2024, 11, 20, 0, 0, 0, 0, time.UTC),
			TypesFound:   []string{"text", "image"},
		},
		{
			Channel:      types.Channel{Name: "#random", ID: "456"},
			MessageCount: 89,
			FirstDate:    time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			LastDate:     time.Date(2024, 10, 5, 0, 0, 0, 0, time.UTC),
			TypesFound:   []string{"text"},
		},
	}

	output := FormatSummaryTable(results)

	// Check content is present
	if !strings.Contains(output, "#general") {
		t.Error("expected #general in output")
	}
	if !strings.Contains(output, "#random") {
		t.Error("expected #random in output")
	}
	if !strings.Contains(output, "342") {
		t.Error("expected message count 342")
	}
	if !strings.Contains(output, "89") {
		t.Error("expected message count 89")
	}
	if !strings.Contains(output, "2022-03-15") {
		t.Error("expected first date")
	}
	if !strings.Contains(output, "Total: 431 messages") {
		t.Error("expected total count")
	}
}

func TestFormatSummaryJSON(t *testing.T) {
	results := []types.ScanResult{
		{
			Channel:      types.Channel{Name: "#general", ID: "123"},
			MessageCount: 100,
			FirstDate:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDate:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			TypesFound:   []string{"text", "image"},
		},
	}

	jsonStr, err := FormatSummaryJSON(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed ScanResultJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Total != 100 {
		t.Errorf("expected total 100, got %d", parsed.Total)
	}
	if len(parsed.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(parsed.Channels))
	}
	if parsed.Channels[0].Name != "#general" {
		t.Errorf("expected channel name #general, got %s", parsed.Channels[0].Name)
	}
}

func TestFormatSummaryJSONEmpty(t *testing.T) {
	jsonStr, err := FormatSummaryJSON(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed ScanResultJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Total != 0 {
		t.Errorf("expected total 0, got %d", parsed.Total)
	}
}

func TestFormatDeleteJSON(t *testing.T) {
	jsonStr, err := FormatDeleteJSON(100, 5, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed DeleteResultJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Deleted != 100 {
		t.Errorf("expected deleted 100, got %d", parsed.Deleted)
	}
	if parsed.Failed != 5 {
		t.Errorf("expected failed 5, got %d", parsed.Failed)
	}
	if parsed.Skipped != 3 {
		t.Errorf("expected skipped 3, got %d", parsed.Skipped)
	}
}

func TestConfirmDeletionSuccess(t *testing.T) {
	reader := strings.NewReader("delete 42\n")
	writer := &bytes.Buffer{}

	err := ConfirmDeletion(42, "discord", "My Server", "", true, reader, writer)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestConfirmDeletionWrongInput(t *testing.T) {
	reader := strings.NewReader("no\n")
	writer := &bytes.Buffer{}

	err := ConfirmDeletion(42, "discord", "My Server", "", false, reader, writer)
	if err == nil {
		t.Error("expected error for wrong input")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected cancellation error, got: %v", err)
	}
}

func TestConfirmDeletionNoInput(t *testing.T) {
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	err := ConfirmDeletion(42, "discord", "My Server", "", false, reader, writer)
	if err == nil {
		t.Error("expected error for no input")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected cancellation error, got: %v", err)
	}
}

func TestConfirmDeletionOutput(t *testing.T) {
	reader := strings.NewReader("delete 10\n")
	writer := &bytes.Buffer{}

	_ = ConfirmDeletion(10, "telegram", "Family Chat", "after:2024-01-01", false, reader, writer)

	output := writer.String()
	if !strings.Contains(output, "WARNING") {
		t.Error("expected WARNING in output")
	}
	if !strings.Contains(output, "telegram") {
		t.Error("expected platform in output")
	}
	if !strings.Contains(output, "Family Chat") {
		t.Error("expected target in output")
	}
	if !strings.Contains(output, "after:2024-01-01") {
		t.Error("expected filters in output")
	}
	if !strings.Contains(output, "IRREVERSIBLE") {
		t.Error("expected IRREVERSIBLE warning")
	}
}

func TestDeleteSummary(t *testing.T) {
	summary := DeleteSummary(100, 5, 3)
	if !strings.Contains(summary, "Deleted 100") {
		t.Error("expected 'Deleted 100' in summary")
	}
	if !strings.Contains(summary, "Failed 5") {
		t.Error("expected 'Failed 5' in summary")
	}
	if !strings.Contains(summary, "Skipped 3") {
		t.Error("expected 'Skipped 3' in summary")
	}
}

func TestDeleteSummaryNoFailures(t *testing.T) {
	summary := DeleteSummary(50, 0, 0)
	if !strings.Contains(summary, "Deleted 50") {
		t.Error("expected 'Deleted 50' in summary")
	}
}
