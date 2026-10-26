package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// SessionCookieName is the name of the cookie that carries the signed session.
const SessionCookieName = "shelfmark_session"

// sessionTTL is how long an issued session remains valid.
const sessionTTL = 24 * time.Hour

// Session is the decoded, verified content of a session cookie.
type Session struct {
	Email string
	Role  Role
}

var (
	// ErrNoSessionSecret indicates SESSION_SECRET is not configured. A missing
	// signing key is a deployment error, not a runtime fallback.
	ErrNoSessionSecret = errors.New("auth: SESSION_SECRET is not set")
	// ErrInvalidSession indicates the cookie is missing, malformed, has an
	// invalid signature, or is expired.
	ErrInvalidSession = errors.New("auth: invalid session")
	// ErrSessionExpired indicates the cookie signature was valid but the
	// session has expired.
	ErrSessionExpired = errors.New("auth: session expired")
)

// sessionSecret returns the configured signing key, or an error if it is unset.
func sessionSecret() ([]byte, error) {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, ErrNoSessionSecret
	}
	return []byte(secret), nil
}

// signPayload computes the base64url-encoded HMAC-SHA256 signature of payload.
func signPayload(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// UserHash returns a stable, non-reversible identifier for email suitable for
// logs: the first 12 hex chars of HMAC-SHA256(lowercased email, SESSION_SECRET).
// It returns "" when the secret is unset, so callers omit the attribute rather
// than hashing with an empty key.
func UserHash(email string) string {
	secret, err := sessionSecret()
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strings.ToLower(email)))
	return hex.EncodeToString(mac.Sum(nil))[:12]
}

// IssueSession sets a signed, stateless session cookie encoding email, role,
// and expiry. The Secure attribute is set when the request arrived over HTTPS.
func IssueSession(w http.ResponseWriter, r *http.Request, email string, role Role) error {
	secret, err := sessionSecret()
	if err != nil {
		return err
	}

	expiry := time.Now().Add(sessionTTL).Unix()
	payload := fmt.Sprintf("%s|%s|%d", email, role, expiry)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := signPayload(secret, payload)
	value := encodedPayload + "." + signature

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	return nil
}

// VerifySession reads the session cookie from the request, checks its HMAC
// signature and expiry, and returns the decoded session.
func VerifySession(r *http.Request) (Session, error) {
	secret, err := sessionSecret()
	if err != nil {
		return Session{}, err
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Session{}, ErrInvalidSession
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return Session{}, ErrInvalidSession
	}
	encodedPayload, signature := parts[0], parts[1]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	payload := string(payloadBytes)

	expectedSignature := signPayload(secret, payload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return Session{}, ErrInvalidSession
	}

	fields := strings.SplitN(payload, "|", 3)
	if len(fields) != 3 {
		return Session{}, ErrInvalidSession
	}
	email, roleStr, expiryStr := fields[0], fields[1], fields[2]

	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	if time.Now().Unix() > expiry {
		return Session{}, ErrSessionExpired
	}

	return Session{Email: email, Role: Role(roleStr)}, nil
}

// ClearSession overwrites the session cookie so the browser discards it.
func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
