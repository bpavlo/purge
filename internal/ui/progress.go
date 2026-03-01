// Package ui provides terminal output components: progress bars, tables, prompts, and summaries.
package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"

	"github.com/pavlo/purge/internal/types"
)

// OutputMode controls how output is rendered.
type OutputMode int

const (
	// ModeNormal renders styled text output with progress bar.
	ModeNormal OutputMode = iota
	// ModeQuiet suppresses progress bars, only shows errors and summary.
	ModeQuiet
	// ModeJSON outputs JSON instead of styled text.
	ModeJSON
)

// Styles defines lipgloss styles used throughout the UI.
type Styles struct {
	Warning lipgloss.Style
	Success lipgloss.Style
	Pause   lipgloss.Style
	Debug   lipgloss.Style
	Bold    lipgloss.Style
}

// NewStyles creates a set of styles, respecting TTY detection.
// If the output is not a TTY, styles are no-ops (no colors).
func NewStyles() Styles {
	if !IsTTY() {
		return Styles{
			Warning: lipgloss.NewStyle(),
			Success: lipgloss.NewStyle(),
			Pause:   lipgloss.NewStyle(),
			Debug:   lipgloss.NewStyle(),
			Bold:    lipgloss.NewStyle(),
		}
	}
	return Styles{
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),  // Red
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // Green
		Pause:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // Yellow
		Debug:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),  // Dim gray
		Bold:    lipgloss.NewStyle().Bold(true),
	}
}

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// ProgressBar wraps schollz/progressbar for deletion progress.
type ProgressBar struct {
	bar    *progressbar.ProgressBar
	mode   OutputMode
	writer io.Writer
	total  int
}

// NewProgressBar creates a new progress bar.
// In Quiet or JSON mode, the bar is suppressed.
func NewProgressBar(total int, description string, mode OutputMode) *ProgressBar {
	pb := &ProgressBar{
		mode:   mode,
		writer: os.Stderr,
		total:  total,
	}

	if mode == ModeQuiet || mode == ModeJSON {
		// Create a silent bar that writes to io.Discard
		pb.bar = progressbar.NewOptions(total,
			progressbar.OptionSetWriter(io.Discard),
		)
		return pb
	}

	opts := []progressbar.Option{
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription(description),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(30),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetPredictTime(true),
	}

	if IsTTY() {
		opts = append(opts,
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
	}

	pb.bar = progressbar.NewOptions(total, opts...)
	return pb
}

// Increment advances the progress bar by one.
func (p *ProgressBar) Increment() {
	_ = p.bar.Add(1)
}

// SetStatus updates the status message (e.g., for rate limit pauses).
func (p *ProgressBar) SetStatus(msg string) {
	if p.mode == ModeQuiet || p.mode == ModeJSON {
		return
	}
	p.bar.Describe(msg)
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	_ = p.bar.Finish()
	if p.mode != ModeQuiet && p.mode != ModeJSON {
		fmt.Fprintln(p.writer)
	}
}

// FormatSummaryTable formats scan results as a styled text table.
func FormatSummaryTable(results []types.ScanResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	styles := NewStyles()
	var b strings.Builder

	header := fmt.Sprintf(" %-25s %-10s %-30s %s", "Channel/Chat", "Messages", "Date Range", "Types")
	b.WriteString(styles.Bold.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("\u2500", 80))
	b.WriteString("\n")

	total := 0
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
		total += r.MessageCount
	}

	b.WriteString(strings.Repeat("\u2500", 80))
	b.WriteString("\n")
	b.WriteString(styles.Bold.Render(fmt.Sprintf(" Total: %d messages", total)))
	b.WriteString("\n")

	return b.String()
}

// ScanResultJSON is the JSON output format for scan results.
type ScanResultJSON struct {
	Channels []ChannelJSON `json:"channels"`
	Total    int           `json:"total"`
}

// ChannelJSON represents a channel in JSON output.
type ChannelJSON struct {
	Name         string   `json:"name"`
	ID           string   `json:"id"`
	MessageCount int      `json:"message_count"`
	FirstDate    string   `json:"first_date"`
	LastDate     string   `json:"last_date"`
	Types        []string `json:"types"`
}

// FormatSummaryJSON formats scan results as JSON.
func FormatSummaryJSON(results []types.ScanResult) (string, error) {
	out := ScanResultJSON{
		Channels: make([]ChannelJSON, 0, len(results)),
	}
	for _, r := range results {
		out.Channels = append(out.Channels, ChannelJSON{
			Name:         r.Channel.Name,
			ID:           r.Channel.ID,
			MessageCount: r.MessageCount,
			FirstDate:    r.FirstDate.Format("2006-01-02"),
			LastDate:     r.LastDate.Format("2006-01-02"),
			Types:        r.TypesFound,
		})
		out.Total += r.MessageCount
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshal scan results: %w", err)
	}
	return string(data), nil
}

// DeleteResultJSON is the JSON output format for delete results.
type DeleteResultJSON struct {
	Deleted int `json:"deleted"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// FormatDeleteJSON formats delete results as JSON.
func FormatDeleteJSON(deleted, failed, skipped int) (string, error) {
	data, err := json.Marshal(DeleteResultJSON{
		Deleted: deleted,
		Failed:  failed,
		Skipped: skipped,
	})
	if err != nil {
		return "", fmt.Errorf("marshal delete results: %w", err)
	}
	return string(data), nil
}

// ConfirmDeletion prompts the user to confirm a destructive delete operation.
// The user must type "delete {count}" to confirm. Returns nil on confirmation,
// or an error if cancelled.
func ConfirmDeletion(count int, platform string, target string, filters string, archived bool, reader io.Reader, writer io.Writer) error {
	styles := NewStyles()

	archiveStatus := "disabled"
	if archived {
		archiveStatus = "enabled"
	}

	if filters == "" {
		filters = "none (all messages)"
	}

	fmt.Fprintln(writer)
	fmt.Fprintln(writer, styles.Warning.Render(fmt.Sprintf("  WARNING: You are about to delete %d messages.", count)))
	fmt.Fprintf(writer, "  Platform:  %s\n", platform)
	if target != "" {
		fmt.Fprintf(writer, "  Target:    %s\n", target)
	}
	fmt.Fprintf(writer, "  Filters:   %s\n", filters)
	fmt.Fprintf(writer, "  Archive:   %s\n", archiveStatus)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, styles.Warning.Render("  This action is IRREVERSIBLE."))
	fmt.Fprintln(writer)
	fmt.Fprintf(writer, "  Type 'delete %d' to confirm: ", count)

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

// DeleteSummary formats the final delete operation report with colors.
func DeleteSummary(deleted, failed, skipped int) string {
	styles := NewStyles()

	parts := []string{
		styles.Success.Render(fmt.Sprintf("Deleted %d", deleted)),
	}
	if failed > 0 {
		parts = append(parts, styles.Warning.Render(fmt.Sprintf("Failed %d", failed)))
	} else {
		parts = append(parts, fmt.Sprintf("Failed %d", failed))
	}
	if skipped > 0 {
		parts = append(parts, styles.Pause.Render(fmt.Sprintf("Skipped %d", skipped)))
	} else {
		parts = append(parts, fmt.Sprintf("Skipped %d", skipped))
	}

	return strings.Join(parts, ". ") + "."
}
