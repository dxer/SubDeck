package middleware

import (
	"testing"
	"time"
)

func TestLoginLimiterResetsAfterExpiry(t *testing.T) {
	ip := "203.0.113.99"
	ResetLoginFailure(ip)
	t.Cleanup(func() { ResetLoginFailure(ip) })

	for i := 0; i < 5; i++ {
		RecordLoginFailure(ip)
	}
	if err := CheckLoginRateLimit(ip); err == nil {
		t.Fatalf("expected IP to be locked after 5 failures")
	}

	// Simulate expired lock window
	globalLoginLimiter.mu.Lock()
	globalLoginLimiter.lockedUntil[ip] = time.Now().Add(-time.Second)
	globalLoginLimiter.mu.Unlock()

	if err := CheckLoginRateLimit(ip); err != nil {
		t.Fatalf("expected lock to be expired, got %v", err)
	}

	// A single failure after expiry should not immediately re-lock
	RecordLoginFailure(ip)
	if err := CheckLoginRateLimit(ip); err != nil {
		t.Fatalf("expected counter to reset after expiry, got %v", err)
	}
}
