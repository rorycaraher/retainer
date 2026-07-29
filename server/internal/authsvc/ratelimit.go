package authsvc

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// Base request-rate limiting per IP (loose — the backoff lockout below does
	// the real work of slowing down a determined attacker).
	requestsPerSecond = 1
	burstSize         = 5

	failuresBeforeLockout = 5
	initialLockout        = 30 * time.Second
	maxLockout            = 30 * time.Minute
)

type ipState struct {
	limiter     *rate.Limiter
	failures    int
	lockedUntil time.Time
}

// LoginLimiter tracks per-IP request rate and failed-attempt backoff for the
// login endpoint. Zero value is not usable; use NewLoginLimiter.
type LoginLimiter struct {
	mu   sync.Mutex
	byIP map[string]*ipState
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{byIP: make(map[string]*ipState)}
}

func (l *LoginLimiter) state(ip string) *ipState {
	st, ok := l.byIP[ip]
	if !ok {
		st = &ipState{limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burstSize)}
		l.byIP[ip] = st
	}
	return st
}

// Allow reports whether a login attempt from ip may proceed right now. If not,
// it returns the duration the caller should wait before retrying.
func (l *LoginLimiter) Allow(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	st := l.state(ip)
	if now.Before(st.lockedUntil) {
		return false, st.lockedUntil.Sub(now)
	}
	if !st.limiter.AllowN(now, 1) {
		return false, time.Second
	}
	return true, 0
}

// RecordFailure registers a failed login attempt from ip, applying exponential
// backoff lockout once the failure count crosses the threshold.
func (l *LoginLimiter) RecordFailure(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	st := l.state(ip)
	st.failures++
	if st.failures < failuresBeforeLockout {
		return
	}
	shift := st.failures - failuresBeforeLockout
	lockout := initialLockout
	for i := 0; i < shift; i++ {
		lockout *= 2
		if lockout >= maxLockout {
			lockout = maxLockout
			break
		}
	}
	st.lockedUntil = now.Add(lockout)
}

// RecordSuccess clears failure/lockout state for ip after a successful login.
func (l *LoginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byIP, ip)
}
