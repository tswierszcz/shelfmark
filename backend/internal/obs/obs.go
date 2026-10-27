// Package obs holds the shared observability primitives: the JSON logger,
// the request-scoped RequestInfo, and helpers that log an event with the
// request's identifying fields attached.
package obs

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

// Version identifies the running release. It is overridden at build time via
// -ldflags "-X github.com/tswierszcz/shelfmark/backend/internal/obs.Version=<sha>".
var Version = "dev"

// NewLogger returns a JSON logger at Info level that tags every line with the
// service name and release version.
func NewLogger(w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler).With("service", "shelfmark", "version", Version)
}

// requestInfoKey is an unexported type to avoid collisions with context keys
// defined in other packages.
type requestInfoKey struct{}

// RequestInfo carries the request-scoped fields every log line about a request
// should include. ID, Method and Route are set once by Middleware; Role and
// User are filled later by the auth layer through SetUser. A nil *RequestInfo
// is valid: its methods are no-ops, so code running without Middleware (unit
// tests, background jobs) can call them freely.
type RequestInfo struct {
	ID     string
	Method string
	Route  string

	mu   sync.Mutex
	role string
	user string
}

// SetUser records the authenticated caller's role and hashed identity.
func (ri *RequestInfo) SetUser(role, userHash string) {
	if ri == nil {
		return
	}
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.role = role
	ri.user = userHash
}

// Role returns the caller's role recorded by SetUser.
func (ri *RequestInfo) Role() string {
	if ri == nil {
		return ""
	}
	ri.mu.Lock()
	defer ri.mu.Unlock()
	return ri.role
}

// User returns the caller's hashed identity recorded by SetUser.
func (ri *RequestInfo) User() string {
	if ri == nil {
		return ""
	}
	ri.mu.Lock()
	defer ri.mu.Unlock()
	return ri.user
}

// Attrs returns the non-empty fields as slog key/value pairs.
func (ri *RequestInfo) Attrs() []any {
	if ri == nil {
		return nil
	}
	ri.mu.Lock()
	defer ri.mu.Unlock()
	attrs := make([]any, 0, 10)
	for _, kv := range [][2]string{
		{"request_id", ri.ID},
		{"method", ri.Method},
		{"route", ri.Route},
		{"role", ri.role},
		{"user", ri.user},
	} {
		if kv[1] != "" {
			attrs = append(attrs, kv[0], kv[1])
		}
	}
	return attrs
}

// WithRequestInfo returns a copy of ctx carrying ri.
func WithRequestInfo(ctx context.Context, ri *RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey{}, ri)
}

// FromContext returns the RequestInfo stored in ctx by Middleware, or nil when
// absent. The nil value is safe to use: SetUser and Attrs are no-ops on it.
func FromContext(ctx context.Context) *RequestInfo {
	if ctx == nil {
		return nil
	}
	ri, _ := ctx.Value(requestInfoKey{}).(*RequestInfo)
	return ri
}

// LogError logs msg at Error level with the request's attributes, the full
// error chain under "error", and any extra attrs.
func LogError(r *http.Request, msg string, err error, attrs ...any) {
	logWithRequest(r, slog.LevelError, msg, err, attrs)
}

// LogWarn is the Warn-level twin of LogError.
func LogWarn(r *http.Request, msg string, err error, attrs ...any) {
	logWithRequest(r, slog.LevelWarn, msg, err, attrs)
}

func logWithRequest(r *http.Request, level slog.Level, msg string, err error, extra []any) {
	ctx := context.Background()
	var args []any
	if r != nil {
		ctx = r.Context()
		args = append(args, FromContext(ctx).Attrs()...)
	}
	if err != nil {
		args = append(args, "error", err.Error())
	}
	args = append(args, extra...)
	slog.Default().Log(ctx, level, msg, args...)
}
