package obs

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

// flyRequestIDPattern bounds which upstream request ids are trusted for reuse.
var flyRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// Middleware wraps mux so every request gets a request id, a RequestInfo in
// its context, panic recovery, and an access-log line. Access lines are
// written for every /api/ path, and for other paths only when status >= 500.
func Middleware(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		id := r.Header.Get("Fly-Request-Id")
		if !flyRequestIDPattern.MatchString(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)

		_, route := mux.Handler(r)
		ri := &RequestInfo{ID: id, Method: r.Method, Route: route}
		r = r.WithContext(WithRequestInfo(r.Context(), ri))

		sw := &statusWriter{ResponseWriter: w}

		defer func() {
			// abort is set when a handler panicked after its headers were
			// sent: the response can no longer become a 500, so after the
			// access line the connection is aborted (as net/http would do)
			// instead of ending a truncated body cleanly.
			abort := false
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				args := append(ri.Attrs(), "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
				slog.Default().ErrorContext(r.Context(), "panic", args...)
				if !sw.wroteHeader {
					sw.Header().Set("Content-Type", "application/json")
					sw.WriteHeader(http.StatusInternalServerError)
					_, _ = sw.Write([]byte(`{"error":"internal server error"}` + "\n"))
				} else {
					sw.status = http.StatusInternalServerError
					abort = true
				}
			}

			status := sw.statusCode()
			if strings.HasPrefix(r.URL.Path, "/api/") || status >= http.StatusInternalServerError {
				args := append(ri.Attrs(),
					"status", status,
					"duration_ms", time.Since(start).Milliseconds(),
				)
				slog.Default().InfoContext(r.Context(), "request", args...)
			}
			if abort {
				panic(http.ErrAbortHandler)
			}
		}()

		mux.ServeHTTP(sw, r)
	})
}

// newRequestID returns 16 random hex characters.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// statusWriter records the response status for the access log.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.status = http.StatusOK
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (sw *statusWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}

func (sw *statusWriter) statusCode() int {
	if sw.status == 0 {
		return http.StatusOK
	}
	return sw.status
}
