package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
)

// OutputFormat defines the format for console output.
type OutputFormat string

const (
	// FormatJSON outputs ticks as pretty-printed JSON.
	FormatJSON OutputFormat = "json"
	// FormatCompact outputs ticks as compact JSON (one line per tick).
	FormatCompact OutputFormat = "compact"
	// FormatTable outputs ticks in a human-readable table format.
	FormatTable OutputFormat = "table"
)

// ConsoleWriter writes ticks to stdout for debugging purposes.
// It implements the Writer interface and is safe for concurrent use.
type ConsoleWriter struct {
	format OutputFormat
	output io.Writer
	mu     sync.Mutex // Protects concurrent writes
}

// ConsoleWriterOption is a functional option for configuring ConsoleWriter.
type ConsoleWriterOption func(*ConsoleWriter)

// WithFormat sets the output format for the console writer.
func WithFormat(format OutputFormat) ConsoleWriterOption {
	return func(w *ConsoleWriter) {
		w.format = format
	}
}

// WithOutput sets the output destination (useful for testing).
func WithOutput(output io.Writer) ConsoleWriterOption {
	return func(w *ConsoleWriter) {
		w.output = output
	}
}

// NewConsoleWriter creates a new console writer with the specified options.
func NewConsoleWriter(opts ...ConsoleWriterOption) *ConsoleWriter {
	w := &ConsoleWriter{
		format: FormatJSON, // Default to JSON
		output: os.Stdout,  // Default to stdout
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Write writes a single tick to the console.
func (w *ConsoleWriter) Write(ctx context.Context, tick *domain.Tick) error {
	if tick == nil {
		return fmt.Errorf("tick cannot be nil")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.writeTick(tick)
}

// WriteBatch writes multiple ticks to the console.
func (w *ConsoleWriter) WriteBatch(ctx context.Context, ticks []*domain.Tick) error {
	if len(ticks) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, tick := range ticks {
		if err := w.writeTick(tick); err != nil {
			return fmt.Errorf("failed to write tick %d: %w", tick.TickNumber, err)
		}
	}

	return nil
}

// Close is a no-op for console writer as there are no resources to release.
func (w *ConsoleWriter) Close() error {
	return nil
}

// writeTick writes a single tick in the configured format.
// Must be called with w.mu held.
func (w *ConsoleWriter) writeTick(tick *domain.Tick) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(tick, true)
	case FormatCompact:
		return w.writeJSON(tick, false)
	case FormatTable:
		return w.writeTable(tick)
	default:
		return fmt.Errorf("unknown output format: %s", w.format)
	}
}

// writeJSON writes a tick as JSON.
func (w *ConsoleWriter) writeJSON(tick *domain.Tick, pretty bool) error {
	var data []byte
	var err error

	if pretty {
		data, err = json.MarshalIndent(tick, "", "  ")
	} else {
		data, err = json.Marshal(tick)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal tick: %w", err)
	}

	_, err = fmt.Fprintln(w.output, string(data))
	return err
}

// writeTable writes a tick in a human-readable table format.
func (w *ConsoleWriter) writeTable(tick *domain.Tick) error {
	_, err := fmt.Fprintf(w.output, `
┌─────────────────────────────────────────────────────────────────
│ Tick #%d
├─────────────────────────────────────────────────────────────────
│ Timestamp:       %s
│ Batch Hash:      %s
│ VDF Output:      %s
│ VDF Iterations:  %d
│ Transactions:    %d
│ Received At:     %s
└─────────────────────────────────────────────────────────────────
`,
		tick.TickNumber,
		tick.Timestamp.Format("2006-01-02 15:04:05.000000"),
		truncate(tick.BatchHash, 32),
		truncate(tick.VDFProof.Output, 32),
		tick.VDFProof.Iterations,
		len(tick.Transactions),
		tick.ReceivedAt.Format("2006-01-02 15:04:05.000000"),
	)

	return err
}

// truncate truncates a string to maxLen characters and adds "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
