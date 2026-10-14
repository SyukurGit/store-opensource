package handler

import (
	"sync"
	"time"
)

// LoginAttemptTracker tracks failed login attempts and applies temporary bans
type LoginAttemptTracker struct {
	mu          sync.Mutex
	failures    map[string][]time.Time // clientIP -> list of timestamps
	bannedUntil map[string]time.Time   // clientIP -> unban timestamp
	maxAttempts int                    // default: 10
	window      time.Duration          // default: 15 minutes
	banDuration time.Duration          // default: 15 minutes

	// Global rapid failure tracking for distributed botnet throttling
	globalFailures []time.Time
}

// NewLoginAttemptTracker creates a new thread-safe rate limiter
func NewLoginAttemptTracker(maxAttempts int, window, banDuration time.Duration) *LoginAttemptTracker {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	if banDuration <= 0 {
		banDuration = 15 * time.Minute
	}

	return &LoginAttemptTracker{
		failures:       make(map[string][]time.Time),
		bannedUntil:    make(map[string]time.Time),
		maxAttempts:    maxAttempts,
		window:         window,
		banDuration:    banDuration,
		globalFailures: []time.Time{},
	}
}

// IsBanned checks if an IP is currently banned. Returns true and the remaining ban duration.
func (t *LoginAttemptTracker) IsBanned(ip string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if unbanTime, exists := t.bannedUntil[ip]; exists {
		if now.Before(unbanTime) {
			return true, unbanTime.Sub(now)
		}
		// Ban has expired, clean up
		delete(t.bannedUntil, ip)
		delete(t.failures, ip)
	}
	return false, 0
}

// RecordFailure records a failed login attempt for an IP.
// Returns isNowBanned (bool), failCount (int within window), and banRemaining (time.Duration).
func (t *LoginAttemptTracker) RecordFailure(ip string) (bool, int, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-t.window)

	// Filter timestamps within window
	var valid []time.Time
	for _, ts := range t.failures[ip] {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	valid = append(valid, now)
	t.failures[ip] = valid

	// Also record in global failure window (last 60 seconds)
	globalCutoff := now.Add(-60 * time.Second)
	var validGlobal []time.Time
	for _, ts := range t.globalFailures {
		if ts.After(globalCutoff) {
			validGlobal = append(validGlobal, ts)
		}
	}
	validGlobal = append(validGlobal, now)
	t.globalFailures = validGlobal

	count := len(valid)
	if count >= t.maxAttempts {
		unban := now.Add(t.banDuration)
		t.bannedUntil[ip] = unban
		return true, count, t.banDuration
	}

	return false, count, 0
}

// ShouldThrottleGlobal checks if there is a rapid distributed attack (>15 failures in 60 seconds)
func (t *LoginAttemptTracker) ShouldThrottleGlobal() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-60 * time.Second)
	count := 0
	for _, ts := range t.globalFailures {
		if ts.After(cutoff) {
			count++
		}
	}
	return count >= 15
}

// Reset clears failure tracking for an IP upon successful login
func (t *LoginAttemptTracker) Reset(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, ip)
	delete(t.bannedUntil, ip)
}
