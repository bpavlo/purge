// Package ui provides terminal output components: progress bars, tables, prompts, and summaries.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/purge/purge/internal/types"
)

// OutputMode controls how output is rendered.
type OutputMode int

const (
	// ModeNormal renders styled text output.
	ModeNormal OutputMode = iota
	// ModeQuiet suppresses progress bars, only shows errors and summary.
	ModeQuiet
	// ModeJSON outputs JSON instead of styled text.
	ModeJSON
)

// ProgressBar wraps a simple terminal progress indicator.
type ProgressBar struct {
	total       int
	current     int
	description string
	status      string
	mode        OutputMode
	writer      io.Writer
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total int, description string, mode OutputMode) *ProgressBar {
	return &ProgressBar{
		total:       total,
		description: description,
		mode:        mode,
		writer:      os.Stderr,
	}
}

// Increment advances the progress bar by one.
func (p *ProgressBar) Increment() {
	p.current++
	if p.mode == ModeQuiet || p.mode == ModeJSON {
		return
	}
	pct := 0
	if p.total > 0 {
		pct = (p.current * 100) / p.total
	}
	fmt.Fprintf(p.writer, "\r%s: %d/%d (%d%%)", p.description, p.current, p.total, pct)
}

// SetStatus updates the status message (e.g., for rate limit pauses).
func (p *ProgressBar) SetStatus(msg string) {
	p.status = msg
	if p.mode == ModeQuiet || p.mode == ModeJSON {
		return
	}
	fmt.Fprintf(p.writer, "\r%s: %s", p.description, msg)
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	if p.mode == ModeQuiet || p.mode == ModeJSON {
		return
	}
	fmt.Fprintln(p.writer)
}

// FormatSummaryTable formats scan results as a text table.
func FormatSummaryTable(results []types.ScanResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var b strings.Builder

	header := fmt.Sprintf(" %-25s %-10s %-30s %s", "Channel/Chat", "Messages", "Date Range", "Types")
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("\u2500", 80))
	b.WriteString("\n")

	for _, r := range results {
		dateRange := fmt.Sprintf("%s \u2192 %s",
			r.FirstDate.Format("2006-01-02"),
			r.LastDate.Format("2006-01-02"),
		)
		typesStr := strings.Join(r.TypesFound, ", ")
		line := fmt.Sprintf(" %-25s %-10d %-30s %s",
			r.Channel.Name, r.MessageCount, dateRange, typesStr,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// ConfirmDeletion prompts the user to confirm a destructive delete operation.
// The user must type "delete {count}" to confirm. Returns nil on confirmation,
// or an error if cancelled.
func ConfirmDeletion(count int, platform string, filters string, archived bool, reader io.Reader) error {
	archiveStatus := "NOT archived"
	if archived {
		archiveStatus = "archived"
	}

	fmt.Println()
	fmt.Printf("  WARNING: You are about to delete %d messages\n", count)
	fmt.Printf("  Platform: %s\n", platform)
	if filters != "" {
		fmt.Printf("  Filters: %s\n", filters)
	}
	fmt.Printf("  Archive: %s\n", archiveStatus)
	fmt.Println()
	fmt.Printf("  Type 'delete %d' to confirm: ", count)

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		return fmt.Errorf("cancelled: no input")
	}

	expected := fmt.Sprintf("delete %d", count)
	if strings.TrimSpace(scanner.Text()) != expected {
		return fmt.Errorf("cancelled: confirmation did not match (expected %q)", expected)
	}

	return nil
}

// DeleteSummary formats the final delete operation report.
func DeleteSummary(deleted, failed, skipped int) string {
	return fmt.Sprintf("Deleted %d. Failed %d. Skipped %d.", deleted, failed, skipped)
}
