package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginLimiter prevents brute-force attacks on admin login
type LoginLimiter struct {
	mu           sync.RWMutex
	attempts     map[string]int       // ip -> failed count
	lockedUntil  map[string]time.Time // ip -> lock expiry
	lastActivity map[string]time.Time
}

var globalLoginLimiter = &LoginLimiter{
	attempts:     make(map[string]int),
	lockedUntil:  make(map[string]time.Time),
	lastActivity: make(map[string]time.Time),
}

func init() {
	// Periodic cleanup of expired rate limit entries every 10 minutes
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			now := time.Now()
			globalLoginLimiter.mu.Lock()
			for ip, t := range globalLoginLimiter.lastActivity {
				if now.Sub(t) > 30*time.Minute {
					delete(globalLoginLimiter.attempts, ip)
					delete(globalLoginLimiter.lockedUntil, ip)
					delete(globalLoginLimiter.lastActivity, ip)
				}
			}
			globalLoginLimiter.mu.Unlock()

			globalSubLimiter.mu.Lock()
			for k, t := range globalSubLimiter.windowStart {
				if now.Sub(t) > 2*time.Minute {
					delete(globalSubLimiter.counts, k)
					delete(globalSubLimiter.windowStart, k)
				}
			}
			globalSubLimiter.mu.Unlock()
		}
	}()
}

// CheckLoginRateLimit verifies if the client IP is temporarily locked out
func CheckLoginRateLimit(ip string) error {
	globalLoginLimiter.mu.RLock()
	defer globalLoginLimiter.mu.RUnlock()

	if until, ok := globalLoginLimiter.lockedUntil[ip]; ok {
		if time.Now().Before(until) {
			remaining := time.Until(until).Round(time.Second)
			return fmt.Errorf("密码连续错误过多，该 IP 已临时锁定，请在 %v 后重试", remaining)
		}
	}
	return nil
}

// RecordLoginFailure tracks a failed attempt and locks after 5 failures
func RecordLoginFailure(ip string) {
	globalLoginLimiter.mu.Lock()
	defer globalLoginLimiter.mu.Unlock()

	now := time.Now()
	// Reset stale failure count once a previous lockout window has expired
	if until, ok := globalLoginLimiter.lockedUntil[ip]; ok && !now.Before(until) {
		delete(globalLoginLimiter.lockedUntil, ip)
		globalLoginLimiter.attempts[ip] = 0
	}

	globalLoginLimiter.lastActivity[ip] = now
	globalLoginLimiter.attempts[ip]++

	if globalLoginLimiter.attempts[ip] >= 5 {
		// Lock IP for 15 minutes
		globalLoginLimiter.lockedUntil[ip] = now.Add(15 * time.Minute)
	}
}

// ResetLoginFailure clears failure count on successful login
func ResetLoginFailure(ip string) {
	globalLoginLimiter.mu.Lock()
	defer globalLoginLimiter.mu.Unlock()

	delete(globalLoginLimiter.attempts, ip)
	delete(globalLoginLimiter.lockedUntil, ip)
	delete(globalLoginLimiter.lastActivity, ip)
}

// SubscriptionLimiter protects /api/sub/:token from high-frequency hammering
type SubscriptionLimiter struct {
	mu          sync.RWMutex
	counts      map[string]int       // key -> count
	windowStart map[string]time.Time // key -> start time
}

var globalSubLimiter = &SubscriptionLimiter{
	counts:      make(map[string]int),
	windowStart: make(map[string]time.Time),
}

// SubscriptionRateLimitMiddleware limits requests to max 20 pulls per 30 seconds per token & IP
func SubscriptionRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		if token == "" {
			c.Next()
			return
		}

		ip := c.ClientIP()
		key := fmt.Sprintf("%s:%s", token, ip)
		now := time.Now()

		globalSubLimiter.mu.Lock()
		start, exists := globalSubLimiter.windowStart[key]
		if !exists || now.Sub(start) > 30*time.Second {
			globalSubLimiter.windowStart[key] = now
			globalSubLimiter.counts[key] = 1
			globalSubLimiter.mu.Unlock()
			c.Next()
			return
		}

		globalSubLimiter.counts[key]++
		count := globalSubLimiter.counts[key]
		globalSubLimiter.mu.Unlock()

		// Max 20 requests per 30 seconds
		if count > 20 {
			c.Header("Retry-After", "30")
			c.String(http.StatusTooManyRequests, "429 Too Many Requests: 订阅拉取过于频繁，请稍候再试。")
			c.Abort()
			return
		}

		c.Next()
	}
}
