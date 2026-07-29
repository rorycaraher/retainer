// Package authsvc implements password auth: hashing, sessions, and login rate limiting.
package authsvc

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const configKeyPasswordHash = "password_hash"

// argon2id parameters. Tuned for a low-traffic single-user server, not a
// high-throughput multi-tenant one — higher memory cost than the library
// defaults since login is rare.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword returns an encoded argon2id hash string safe to store in config.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf("argon2id$%d$%d$%d$%s$%s",
		argonTime, argonMemory, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword checks a plaintext password against an encoded hash produced by HashPassword.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		return false, errors.New("unrecognized password hash format")
	}
	var time, memory uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[1], "%d", &time); err != nil {
		return false, err
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &memory); err != nil {
		return false, err
	}
	var threadsInt int
	if _, err := fmt.Sscanf(parts[3], "%d", &threadsInt); err != nil {
		return false, err
	}
	threads = uint8(threadsInt)
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// SetPassword writes a freshly-hashed password into the config table, overwriting any existing one.
func SetPassword(db *sql.DB, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, configKeyPasswordHash, hash)
	return err
}

// PasswordHash returns the stored hash, or "" if none has been set yet.
func PasswordHash(db *sql.DB) (string, error) {
	var hash string
	err := db.QueryRow(`SELECT value FROM config WHERE key = ?`, configKeyPasswordHash).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}
