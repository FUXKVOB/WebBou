package webbou

import (
	"sync"
	"time"
)

// RateLimiter implements token bucket algorithm
type RateLimiter struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64 // tokens per second
	lastRefill     time.Time
	mu             sync.Mutex
	perIPLimiters  map[string]*IPRateLimiter
	cleanupTicker  *time.Ticker
}

type IPRateLimiter struct {
	tokens     float64
	lastAccess time.Time
}

func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	rl := &RateLimiter{
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		refillRate:    refillRate,
		lastRefill:    time.Now(),
		perIPLimiters: make(map[string]*IPRateLimiter),
		cleanupTicker: time.NewTicker(1 * time.Minute),
	}

	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

func (rl *RateLimiter) AllowN(n float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= n {
		rl.tokens -= n
		return true
	}

	return false
}

func (rl *RateLimiter) AllowIP(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.perIPLimiters[ip]
	if !exists {
		limiter = &IPRateLimiter{
			tokens:     rl.maxTokens / 10, // Per-IP limit is 10% of global
			lastAccess: time.Now(),
		}
		rl.perIPLimiters[ip] = limiter
	}

	// Refill IP tokens
	elapsed := time.Since(limiter.lastAccess).Seconds()
	limiter.tokens += elapsed * (rl.refillRate / 10)
	if limiter.tokens > rl.maxTokens/10 {
		limiter.tokens = rl.maxTokens / 10
	}
	limiter.lastAccess = time.Now()

	if limiter.tokens >= 1.0 {
		limiter.tokens -= 1.0
		return true
	}

	return false
}

func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	rl.lastRefill = now
}

func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTicker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, limiter := range rl.perIPLimiters {
			if now.Sub(limiter.lastAccess) > 5*time.Minute {
				delete(rl.perIPLimiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) GetTokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.tokens
}

func (rl *RateLimiter) Stop() {
	rl.cleanupTicker.Stop()
}

// ConnectionRateLimiter limits connections per IP
type ConnectionRateLimiter struct {
	connections map[string]int
	maxPerIP    int
	mu          sync.RWMutex
}

func NewConnectionRateLimiter(maxPerIP int) *ConnectionRateLimiter {
	return &ConnectionRateLimiter{
		connections: make(map[string]int),
		maxPerIP:    maxPerIP,
	}
}

func (crl *ConnectionRateLimiter) AllowConnection(ip string) bool {
	crl.mu.Lock()
	defer crl.mu.Unlock()

	count := crl.connections[ip]
	if count >= crl.maxPerIP {
		return false
	}

	crl.connections[ip]++
	return true
}

func (crl *ConnectionRateLimiter) ReleaseConnection(ip string) {
	crl.mu.Lock()
	defer crl.mu.Unlock()

	if count := crl.connections[ip]; count > 0 {
		crl.connections[ip]--
		if crl.connections[ip] == 0 {
			delete(crl.connections, ip)
		}
	}
}

func (crl *ConnectionRateLimiter) GetConnectionCount(ip string) int {
	crl.mu.RLock()
	defer crl.mu.RUnlock()
	return crl.connections[ip]
}
