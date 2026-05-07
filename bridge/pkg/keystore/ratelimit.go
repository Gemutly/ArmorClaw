package keystore

import (
	"sync"
	"time"
)

type attemptWindow struct {
	count       int
	windowStart time.Time
}

// RateLimiter provides per-identity fixed-window rate limiting.
// It is safe for concurrent use.
type RateLimiter struct {
	mu             sync.Mutex
	maxAttempts    int
	windowDuration time.Duration
	windows        map[string]*attemptWindow
}

// NewRateLimiter creates a RateLimiter that allows maxAttempts per identity
// within each windowDuration.
func NewRateLimiter(maxAttempts int, windowDuration time.Duration) *RateLimiter {
	return &RateLimiter{
		maxAttempts:    maxAttempts,
		windowDuration: windowDuration,
		windows:        make(map[string]*attemptWindow),
	}
}

// Record increments the attempt count for the given identity within the
// current window. If the window has expired, a new window is started.
func (r *RateLimiter) Record(identity string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	w, ok := r.windows[identity]
	if !ok || now.Sub(w.windowStart) >= r.windowDuration {
		r.windows[identity] = &attemptWindow{
			count:       1,
			windowStart: now,
		}
		return
	}
	w.count++
}

// Exceeded returns true if the given identity has exceeded the maximum
// allowed attempts within the current window.
func (r *RateLimiter) Exceeded(identity string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	w, ok := r.windows[identity]
	if !ok {
		return false
	}
	if now.Sub(w.windowStart) >= r.windowDuration {
		return false
	}
	return w.count > r.maxAttempts
}
