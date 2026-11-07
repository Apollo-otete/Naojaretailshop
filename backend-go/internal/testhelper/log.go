// Package testhelper exists for _test.go files in other packages. It is not imported by any
// production code, and its whole purpose is to let a test assert on log output — which is only
// worth doing if the log is a parseable record rather than prose on stdout.
package testhelper

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
)

// Buffer is a concurrency-safe log sink. The readiness handler runs its checkers in goroutines,
// so an ordinary bytes.Buffer would be a data race under `make test-race` rather than a bug.
type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Logger returns a logger at the given level plus the buffer it writes into.
func Logger(t *testing.T, level slog.Level) (*slog.Logger, *Buffer) {
	t.Helper()
	b := &Buffer{}
	l := slog.New(slog.NewJSONHandler(b, &slog.HandlerOptions{Level: level})).
		With(slog.String("service", "naoja-api"))
	return l, b
}

// Records parses every JSON log line written so far.
func (b *Buffer) Records(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range bytes.Split([]byte(b.String()), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("log line is not JSON: %v\nline: %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

// Find returns the first record whose "msg" equals msg, or nil.
func (b *Buffer) Find(t *testing.T, msg string) map[string]any {
	t.Helper()
	for _, r := range b.Records(t) {
		if r["msg"] == msg {
			return r
		}
	}
	return nil
}

// FindAll returns every record whose "msg" equals msg.
func (b *Buffer) FindAll(t *testing.T, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, r := range b.Records(t) {
		if r["msg"] == msg {
			out = append(out, r)
		}
	}
	return out
}

// Field returns a top-level field from the first record with the given msg.
func (b *Buffer) Field(t *testing.T, msg, field string) any {
	t.Helper()
	r := b.Find(t, msg)
	if r == nil {
		t.Fatalf("no log record with msg %q; got:\n%s", msg, b.String())
	}
	return r[field]
}
