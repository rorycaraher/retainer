package authsvc

import (
	"database/sql"
	"testing"
	"time"

	"retainer/server/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB
}

func TestSessionCreateValidateExpireLogout(t *testing.T) {
	dbConn := openTestDB(t)

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	token, expiresAt, err := CreateSession(dbConn, now)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !expiresAt.Equal(now.Add(sessionTTL)) {
		t.Fatalf("expected expiry %v, got %v", now.Add(sessionTTL), expiresAt)
	}

	if err := ValidateSession(dbConn, token, now.Add(time.Hour)); err != nil {
		t.Fatalf("expected valid session, got %v", err)
	}

	// Sliding expiration: far in the future but within a fresh 90d window from
	// the last validation above should still be valid.
	if err := ValidateSession(dbConn, token, now.Add(89*24*time.Hour)); err != nil {
		t.Fatalf("expected sliding expiration to keep session alive, got %v", err)
	}

	// Well past any sliding renewal should expire.
	if err := ValidateSession(dbConn, token, now.Add(400*24*time.Hour)); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid for expired session, got %v", err)
	}

	token2, _, err := CreateSession(dbConn, now)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := DeleteSession(dbConn, token2); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if err := ValidateSession(dbConn, token2, now); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid after logout, got %v", err)
	}
}

func TestValidateUnknownToken(t *testing.T) {
	dbConn := openTestDB(t)
	if err := ValidateSession(dbConn, "not-a-real-token", time.Now()); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid, got %v", err)
	}
}
