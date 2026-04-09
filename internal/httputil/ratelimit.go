package httputil

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig controls the per-client token-bucket rate limiter.
//
// Defaults are intentionally lenient — the goal right now is to blunt runaway
// clients and accidental loops, not to police legitimate traffic. Tighten
// these numbers once we have real usage data.
type RateLimitConfig struct {
	// RPS is the sustained per-client request rate allowed.
	RPS rate.Limit
	// Burst is the maximum short-term burst size.
	Burst int
	// IdleTTL is how long an unused client entry is retained before it
	// becomes eligible for cleanup. Zero falls back to 15 minutes.
	IdleTTL time.Duration
}

// DefaultRateLimit returns the lenient baseline used when no explicit config
// is provided: 100 rps sustained, 200 burst, 15-minute idle cleanup.
func DefaultRateLimit() RateLimitConfig {
	return RateLimitConfig{
		RPS:     100,
		Burst:   200,
		IdleTTL: 15 * time.Minute,
	}
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is a per-IP token-bucket middleware. It is safe for concurrent
// use and cleans up idle clients in the background.
type RateLimiter struct {
	cfg      RateLimitConfig
	mu       sync.Mutex
	clients  map[string]*clientLimiter
	stopOnce sync.Once
	stop     chan struct{}
}

// NewRateLimiter builds a RateLimiter with the given config and starts a
// background cleanup goroutine. Call Close to stop it (tests only — the
// server process exits when the goroutine goes away).
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if cfg.IdleTTL == 0 {
		cfg.IdleTTL = 15 * time.Minute
	}
	rl := &RateLimiter{
		cfg:     cfg,
		clients: make(map[string]*clientLimiter),
		stop:    make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Middleware returns a gin.HandlerFunc that enforces the rate limit per
// client IP. Clients that exceed the bucket receive 429 Too Many Requests.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.limiterFor(c.ClientIP()).Allow() {
			WriteError(c, http.StatusTooManyRequests, "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) limiterFor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cl, ok := rl.clients[ip]
	if !ok {
		cl = &clientLimiter{limiter: rate.NewLimiter(rl.cfg.RPS, rl.cfg.Burst)}
		rl.clients[ip] = cl
	}
	cl.lastSeen = time.Now()
	return cl.limiter
}

func (rl *RateLimiter) cleanupLoop() {
	// Sweep at 1/3 of the idle TTL so entries don't linger much past their
	// window, but we're not burning CPU on a tight loop.
	interval := rl.cfg.IdleTTL / 3
	if interval < time.Minute {
		interval = time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case now := <-t.C:
			rl.mu.Lock()
			for ip, cl := range rl.clients {
				if now.Sub(cl.lastSeen) > rl.cfg.IdleTTL {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Close stops the background cleanup goroutine. Safe to call multiple times.
func (rl *RateLimiter) Close() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}
