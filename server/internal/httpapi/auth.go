package httpapi

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"retainer/server/internal/authsvc"
)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     authsvc.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authsvc.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func handleLogin(db *sql.DB, limiter *authsvc.LoginLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		now := time.Now()

		if ok, wait := limiter.Allow(ip, now); !ok {
			secs := int(wait.Seconds())
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
			return
		}

		var in struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		hash, err := authsvc.PasswordHash(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if hash == "" {
			writeError(w, http.StatusServiceUnavailable, "server has no password configured")
			return
		}

		ok, err := authsvc.VerifyPassword(in.Password, hash)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			limiter.RecordFailure(ip, now)
			writeError(w, http.StatusUnauthorized, "invalid password")
			return
		}

		limiter.RecordSuccess(ip)
		token, expiresAt, err := authsvc.CreateSession(db, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		setSessionCookie(w, token, expiresAt)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func handleLogout(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(authsvc.CookieName); err == nil {
			_ = authsvc.DeleteSession(db, c.Value)
		}
		clearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func handleAuthStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"loggedIn": true})
	}
}

// requireAuth wraps a handler so it 401s unless the request carries a valid
// session cookie. Sliding expiration is applied as a side effect of validation.
func requireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(authsvc.CookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		if err := authsvc.ValidateSession(db, c.Value, time.Now()); err != nil {
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		next.ServeHTTP(w, r)
	})
}
