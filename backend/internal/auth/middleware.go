package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

// sessionContextKey is an unexported type to avoid collisions with context
// keys defined in other packages.
type sessionContextKey struct{}

// SessionFromContext returns the Session attached to ctx by RequireRole, and
// whether one was present.
func SessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(Session)
	return session, ok
}

// RequireRole returns a decorator that only runs the wrapped handler when the
// caller's session is valid and its role is one of allowed. It responds with
// 401 when the session is missing, malformed, or expired, and 403 when the
// session is valid but its role is not permitted. On success, the verified
// Session is attached to the request context for the wrapped handler to read
// via SessionFromContext, and the caller's role and hashed identity are
// recorded on the request's obs.RequestInfo for logging.
func RequireRole(allowed ...Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			session, err := VerifySession(r)
			if err != nil {
				// A missing signing key and forged/rotated-key cookies are
				// operational problems, not ordinary logouts, so they are
				// logged; every other failure (no cookie, malformed, expired)
				// is routine and stays silent.
				switch {
				case errors.Is(err, ErrNoSessionSecret):
					obs.LogError(r, "session secret not configured", err)
				case errors.Is(err, ErrInvalidSignature):
					obs.LogWarn(r, "session signature invalid", err)
				}
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			permitted := false
			for _, role := range allowed {
				if session.Role == role {
					permitted = true
					break
				}
			}
			if !permitted {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			obs.FromContext(r.Context()).SetUser(string(session.Role), UserHash(session.Email))

			ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
			next(w, r.WithContext(ctx))
		}
	}
}
