// Package obstest captures slog output in tests so they can assert on the
// JSON log lines a piece of code emits.
package obstest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// Sink is a mutex-guarded writer that collects JSON log lines. Workers left
// running by earlier tests may also write to it, so assertions should filter
// by the test's own request_id / library_id rather than count all lines.
type Sink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *Sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

// Lines returns every captured log line decoded as a JSON object.
func (s *Sink) Lines() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines := make([]map[string]any, 0)
	for _, raw := range strings.Split(s.buf.String(), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			lines = append(lines, m)
		}
	}
	return lines
}

// Find returns the lines whose msg equals msg (any msg when msg is empty) and
// whose attributes match every key/value pair in attrs (values compared via
// fmt.Sprint).
func (s *Sink) Find(msg string, attrs ...any) []map[string]any {
	found := make([]map[string]any, 0)
	for _, line := range s.Lines() {
		if msg != "" && line["msg"] != msg {
			continue
		}
		match := true
		for i := 0; i+1 < len(attrs); i += 2 {
			if fmt.Sprint(line[fmt.Sprint(attrs[i])]) != fmt.Sprint(attrs[i+1]) {
				match = false
				break
			}
		}
		if match {
			found = append(found, line)
		}
	}
	return found
}

// Capture installs a plain JSON logger over a new Sink as the slog default
// for the duration of the test and restores the previous default on cleanup.
func Capture(t testing.TB) *Sink {
	t.Helper()
	return CaptureWith(t, func(w io.Writer) *slog.Logger {
		return slog.New(slog.NewJSONHandler(w, nil))
	})
}

// CaptureWith is Capture with a caller-supplied logger constructor, so tests
// can capture through the production logger (obs.NewLogger) without this
// package importing obs.
func CaptureWith(t testing.TB, newLogger func(io.Writer) *slog.Logger) *Sink {
	t.Helper()
	sink := &Sink{}
	prev := slog.Default()
	slog.SetDefault(newLogger(sink))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return sink
}
