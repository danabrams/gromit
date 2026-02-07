package runner

import (
	"bytes"
	"sync"
	"testing"
)

func TestSyncWriter_Write(t *testing.T) {
	tests := []struct {
		name string
		ops  []writeOp
		want string
	}{
		{
			name: "single write",
			ops: []writeOp{
				{data: []byte("hello\n"), isOverwrite: false},
			},
			want: "hello\n",
		},
		{
			name: "multiple writes",
			ops: []writeOp{
				{data: []byte("line1\n"), isOverwrite: false},
				{data: []byte("line2\n"), isOverwrite: false},
			},
			want: "line1\nline2\n",
		},
		{
			name: "overwrite followed by normal write",
			ops: []writeOp{
				{data: []byte("\r[0m15s] Waiting..."), isOverwrite: true},
				{data: []byte("Claude text"), isOverwrite: false},
			},
			want: "\r[0m15s] Waiting...\nClaude text",
		},
		{
			name: "multiple overwrites followed by normal write",
			ops: []writeOp{
				{data: []byte("\r[0m15s] Waiting..."), isOverwrite: true},
				{data: []byte("\r[0m20s] 3 tool calls"), isOverwrite: true},
				{data: []byte("Claude text"), isOverwrite: false},
			},
			want: "\r[0m15s] Waiting...\r[0m20s] 3 tool calls\nClaude text",
		},
		{
			name: "normal write after overwrite after normal write",
			ops: []writeOp{
				{data: []byte("setup\n"), isOverwrite: false},
				{data: []byte("\r[0m15s] Waiting..."), isOverwrite: true},
				{data: []byte("Claude text"), isOverwrite: false},
			},
			want: "setup\n\r[0m15s] Waiting...\nClaude text",
		},
		{
			name: "consecutive normal writes after overwrite",
			ops: []writeOp{
				{data: []byte("\r[0m15s] Waiting..."), isOverwrite: true},
				{data: []byte("text1"), isOverwrite: false},
				{data: []byte("text2"), isOverwrite: false},
			},
			want: "\r[0m15s] Waiting...\ntext1text2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			sw := newSyncWriter(&buf)

			for _, op := range tt.ops {
				var err error
				if op.isOverwrite {
					_, err = sw.WriteOverwrite(op.data)
				} else {
					_, err = sw.Write(op.data)
				}
				if err != nil {
					t.Fatalf("write failed: %v", err)
				}
			}

			got := buf.String()
			if got != tt.want {
				t.Errorf("output mismatch\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestSyncWriter_Concurrent(t *testing.T) {
	// Test that concurrent writes are properly serialized
	var buf bytes.Buffer
	sw := newSyncWriter(&buf)

	const numWriters = 10
	const writesPerWriter = 100

	var wg sync.WaitGroup
	wg.Add(numWriters)

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				// Alternate between normal writes and overwrites
				if j%2 == 0 {
					sw.Write([]byte("a"))
				} else {
					sw.WriteOverwrite([]byte("b"))
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify that we got the expected number of bytes written
	// (we don't check exact order due to concurrency, but we can verify
	// that all writes completed without data loss or corruption)
	totalBytes := buf.Len()
	if totalBytes == 0 {
		t.Error("expected some bytes to be written")
	}

	// Each write is 1 byte, and some normal writes after overwrites
	// will have an extra newline prepended. The total should be at least
	// the number of writes (numWriters * writesPerWriter).
	minExpected := numWriters * writesPerWriter
	if totalBytes < minExpected {
		t.Errorf("expected at least %d bytes, got %d", minExpected, totalBytes)
	}
}

func TestSyncWriter_StateReset(t *testing.T) {
	// Test that lastWasOverwrite is properly reset after a normal write
	var buf bytes.Buffer
	sw := newSyncWriter(&buf)

	// Overwrite
	sw.WriteOverwrite([]byte("overwrite"))
	// Normal write (should add newline)
	sw.Write([]byte("text1"))
	// Another normal write (should NOT add newline)
	sw.Write([]byte("text2"))

	want := "overwrite\ntext1text2"
	got := buf.String()
	if got != want {
		t.Errorf("state not reset properly\ngot:  %q\nwant: %q", got, want)
	}
}

// writeOp represents a write operation for testing
type writeOp struct {
	data        []byte
	isOverwrite bool
}
