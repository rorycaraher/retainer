package authsvc

import (
	"testing"
	"time"
)

func TestLoginLimiterLockoutAfterFailures(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	ip := "10.0.0.1"

	for i := 0; i < failuresBeforeLockout; i++ {
		l.RecordFailure(ip, now)
	}

	ok, wait := l.Allow(ip, now)
	if ok {
		t.Fatal("expected lockout after threshold failures")
	}
	if wait < initialLockout-time.Second {
		t.Fatalf("expected wait around %v, got %v", initialLockout, wait)
	}

	// Still locked out just before the lockout window ends.
	ok, _ = l.Allow(ip, now.Add(initialLockout-time.Second))
	if ok {
		t.Fatal("expected still locked out")
	}

	// After the window, allowed again.
	ok, _ = l.Allow(ip, now.Add(initialLockout+time.Second))
	if !ok {
		t.Fatal("expected lockout to have expired")
	}
}

func TestLoginLimiterSuccessResetsFailures(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	ip := "10.0.0.2"

	for i := 0; i < failuresBeforeLockout-1; i++ {
		l.RecordFailure(ip, now)
	}
	l.RecordSuccess(ip)

	l.RecordFailure(ip, now)
	ok, _ := l.Allow(ip, now)
	if !ok {
		t.Fatal("expected no lockout after success reset failure count")
	}
}

func TestLoginLimiterLockoutGrowsAndCaps(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	ip := "10.0.0.3"

	for i := 0; i < failuresBeforeLockout+3; i++ {
		l.RecordFailure(ip, now)
	}
	st := l.byIP[ip]
	if st.lockedUntil.Sub(now) > maxLockout {
		t.Fatalf("expected lockout capped at %v, got %v", maxLockout, st.lockedUntil.Sub(now))
	}
}
