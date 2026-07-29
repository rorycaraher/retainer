package authsvc

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const (
	tokenBytes = 32
	sessionTTL = 90 * 24 * time.Hour
	CookieName = "retainer_session"
)

// ErrSessionInvalid is returned when a token has no matching, unexpired session.
var ErrSessionInvalid = errors.New("invalid or expired session")

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession issues a new opaque session token and stores its hash.
func CreateSession(db *sql.DB, now time.Time) (token string, expiresAt time.Time, err error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expiresAt = now.Add(sessionTTL)

	_, err = db.Exec(`INSERT INTO sessions (token_hash, created_at, expires_at, last_used_at) VALUES (?, ?, ?, ?)`,
		hashToken(token), now.UnixMilli(), expiresAt.UnixMilli(), now.UnixMilli())
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// ValidateSession checks a token, and if valid, slides its expiration forward from now.
// Returns ErrSessionInvalid if the token is unknown or expired.
func ValidateSession(db *sql.DB, token string, now time.Time) error {
	hash := hashToken(token)
	var expiresAt int64
	err := db.QueryRow(`SELECT expires_at FROM sessions WHERE token_hash = ?`, hash).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionInvalid
	}
	if err != nil {
		return err
	}
	if now.UnixMilli() > expiresAt {
		return ErrSessionInvalid
	}

	newExpiry := now.Add(sessionTTL).UnixMilli()
	_, err = db.Exec(`UPDATE sessions SET last_used_at = ?, expires_at = ? WHERE token_hash = ?`,
		now.UnixMilli(), newExpiry, hash)
	return err
}

// DeleteSession revokes a session (logout).
func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}
