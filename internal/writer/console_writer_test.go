package writer

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
)

func createTestTick() *domain.Tick {
	return &domain.Tick{
		TickNumber: 12345,
		Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		VDFProof: domain.VDFProof{
			Input:      "input123",
			Output:     "output456",
			Proof:      "proof789",
			Iterations: 1000,
		},
		BatchHash:    "batch_hash_123",
		PrevOutput:   "prev_output_456",
		ReceivedAt:   time.Date(2025, 1, 1, 12, 0, 1, 0, time.UTC),
		Transactions: []domain.Transaction{},
	}
}

func TestConsoleWriter_Write(t *testing.T) {
	tests := []struct {
		name    string
		tick    *domain.Tick
		format  OutputFormat
		wantErr bool
	}{
		{
			name:    "write valid tick as JSON",
			tick:    createTestTick(),
			format:  FormatJSON,
			wantErr: false,
		},
		{
			name:    "write valid tick as compact JSON",
			tick:    createTestTick(),
			format:  FormatCompact,
			wantErr: false,
		},
		{
			name:    "write valid tick as table",
			tick:    createTestTick(),
			format:  FormatTable,
			wantErr: false,
		},
		{
			name:    "write nil tick",
			tick:    nil,
			format:  FormatJSON,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewConsoleWriter(
				WithFormat(tt.format),
				WithOutput(&buf),
			)

			ctx := context.Background()
			err := writer.Write(ctx, tt.tick)

			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()
				if len(output) == 0 {
					t.Errorf("Write() produced no output")
				}

				// Verify output contains tick number
				if !strings.Contains(output, "12345") {
					t.Errorf("Write() output doesn't contain tick number, got: %s", output)
				}
			}
		})
	}
}

func TestConsoleWriter_WriteBatch(t *testing.T) {
	t.Run("write multiple ticks", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewConsoleWriter(
			WithFormat(FormatCompact),
			WithOutput(&buf),
		)

		ticks := []*domain.Tick{
			createTestTick(),
			func() *domain.Tick {
				tick := createTestTick()
				tick.TickNumber = 12346
				return tick
			}(),
			func() *domain.Tick {
				tick := createTestTick()
				tick.TickNumber = 12347
				return tick
			}(),
		}

		ctx := context.Background()
		err := writer.WriteBatch(ctx, ticks)

		if err != nil {
			t.Fatalf("WriteBatch() error = %v, want nil", err)
		}

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 3 {
			t.Errorf("WriteBatch() produced %d lines, want 3", len(lines))
		}

		// Verify each tick is in the output
		if !strings.Contains(output, "12345") {
			t.Errorf("WriteBatch() output doesn't contain tick 12345")
		}
		if !strings.Contains(output, "12346") {
			t.Errorf("WriteBatch() output doesn't contain tick 12346")
		}
		if !strings.Contains(output, "12347") {
			t.Errorf("WriteBatch() output doesn't contain tick 12347")
		}
	})

	t.Run("write empty batch", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewConsoleWriter(WithOutput(&buf))

		ctx := context.Background()
		err := writer.WriteBatch(ctx, []*domain.Tick{})

		if err != nil {
			t.Errorf("WriteBatch() with empty slice error = %v, want nil", err)
		}

		if buf.Len() > 0 {
			t.Errorf("WriteBatch() with empty slice produced output: %s", buf.String())
		}
	})
}

func TestConsoleWriter_FormatJSON(t *testing.T) {
	var buf bytes.Buffer
	writer := NewConsoleWriter(
		WithFormat(FormatJSON),
		WithOutput(&buf),
	)

	tick := createTestTick()
	ctx := context.Background()

	err := writer.Write(ctx, tick)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	output := buf.String()

	// Verify it's valid JSON
	var parsed domain.Tick
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("Write() produced invalid JSON: %v", err)
	}

	// Verify pretty printing (should have indentation)
	if !strings.Contains(output, "  ") {
		t.Errorf("Write() with FormatJSON should be pretty-printed, got: %s", output)
	}
}

func TestConsoleWriter_FormatCompact(t *testing.T) {
	var buf bytes.Buffer
	writer := NewConsoleWriter(
		WithFormat(FormatCompact),
		WithOutput(&buf),
	)

	tick := createTestTick()
	ctx := context.Background()

	err := writer.Write(ctx, tick)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	output := strings.TrimSpace(buf.String())

	// Verify it's valid JSON
	var parsed domain.Tick
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("Write() produced invalid JSON: %v", err)
	}

	// Verify it's on a single line (no newlines except the trailing one)
	lines := strings.Split(output, "\n")
	if len(lines) != 1 {
		t.Errorf("Write() with FormatCompact should produce single line, got %d lines", len(lines))
	}
}

func TestConsoleWriter_FormatTable(t *testing.T) {
	var buf bytes.Buffer
	writer := NewConsoleWriter(
		WithFormat(FormatTable),
		WithOutput(&buf),
	)

	tick := createTestTick()
	ctx := context.Background()

	err := writer.Write(ctx, tick)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	output := buf.String()

	// Verify table structure
	if !strings.Contains(output, "┌─") {
		t.Errorf("Write() with FormatTable should contain table borders")
	}

	// Verify tick number is present
	if !strings.Contains(output, "Tick #12345") {
		t.Errorf("Write() with FormatTable should contain tick number")
	}

	// Verify key fields are present
	expectedFields := []string{"Timestamp:", "Batch Hash:", "VDF Output:", "Transactions:"}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Write() with FormatTable should contain field %q", field)
		}
	}
}

func TestConsoleWriter_Close(t *testing.T) {
	writer := NewConsoleWriter()

	err := writer.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestConsoleWriter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	writer := NewConsoleWriter(
		WithFormat(FormatCompact),
		WithOutput(&buf),
	)

	ctx := context.Background()
	numGoroutines := 10
	ticksPerGoroutine := 10

	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < ticksPerGoroutine; j++ {
				tick := createTestTick()
				tick.TickNumber = uint64(id*ticksPerGoroutine + j)
				_ = writer.Write(ctx, tick)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify we got output from all writes
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	expectedLines := numGoroutines * ticksPerGoroutine
	if len(lines) != expectedLines {
		t.Errorf("ConcurrentWrites produced %d lines, want %d", len(lines), expectedLines)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "string shorter than maxLen",
			input:  "short",
			maxLen: 10,
			want:   "short",
		},
		{
			name:   "string equal to maxLen",
			input:  "exactly10c",
			maxLen: 10,
			want:   "exactly10c",
		},
		{
			name:   "string longer than maxLen",
			input:  "this is a very long string",
			maxLen: 10,
			want:   "this is a ...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate() = %v, want %v", got, tt.want)
			}
		})
	}
}
