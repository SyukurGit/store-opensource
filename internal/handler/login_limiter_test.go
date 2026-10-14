package handler

import (
	"testing"
	"time"
)

func TestLoginAttemptTracker(t *testing.T) {
	// Create limiter with max 3 attempts for testing, 1 second window & ban
	limiter := NewLoginAttemptTracker(3, 1*time.Second, 1*time.Second)
	testIP := "192.168.1.100"

	// 1. Initial state: not banned
	banned, _ := limiter.IsBanned(testIP)
	if banned {
		t.Fatalf("expected IP not to be banned initially")
	}

	// 2. First failure
	isBanned, count, _ := limiter.RecordFailure(testIP)
	if isBanned || count != 1 {
		t.Fatalf("expected count 1, not banned")
	}

	// 3. Second failure
	isBanned, count, _ = limiter.RecordFailure(testIP)
	if isBanned || count != 2 {
		t.Fatalf("expected count 2, not banned")
	}

	// 4. Third failure -> triggers ban!
	isBanned, count, remaining := limiter.RecordFailure(testIP)
	if !isBanned || count != 3 || remaining <= 0 {
		t.Fatalf("expected IP to be banned after 3 failures")
	}

	// Verify IsBanned
	banned, _ = limiter.IsBanned(testIP)
	if !banned {
		t.Fatalf("expected IsBanned to return true")
	}

	// 5. Reset clears ban and counter
	limiter.Reset(testIP)
	banned, _ = limiter.IsBanned(testIP)
	if banned {
		t.Fatalf("expected IP to be unbanned after Reset")
	}
}
