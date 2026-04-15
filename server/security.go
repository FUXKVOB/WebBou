package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"
)

type SecurityManager struct {
	mu sync.RWMutex
	tokens map[string]*Token
	rateLimiter *RateLimiter
}

type Token struct {
	Value string
	ExpiresAt time.Time
	SessionID string
}

type RateLimiter struct {
	mu sync.RWMutex
	requests map[string]*RequestCounter
	maxRequests int
	window time.Duration
}

type RequestCounter struct {
	Count int
	ResetAt time.Time
}

func NewSecurityManager() *SecurityManager {
	sm := &SecurityManager{
		tokens: make(map[string]*Token),
		rateLimiter: &RateLimiter{
			requests: make(map[string]*RequestCounter),
			maxRequests: 1000,
			window: time.Minute,
		},
	}
	
	go sm.cleanupExpiredTokens()
	return sm
}

func (sm *SecurityManager) GenerateToken(sessionID string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	
	hash := sha256.Sum256(tokenBytes)
	tokenStr := base64.URLEncoding.EncodeToString(hash[:])
	
	token := &Token{
		Value: tokenStr,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		SessionID: sessionID,
	}
	
	sm.mu.Lock()
	sm.tokens[tokenStr] = token
	sm.mu.Unlock()
	
	return tokenStr, nil
}

func (sm *SecurityManager) ValidateToken(tokenStr string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	token, exists := sm.tokens[tokenStr]
	if !exists {
		return false
	}
	
	return time.Now().Before(token.ExpiresAt)
}

func (sm *SecurityManager) CheckRateLimit(clientID string) bool {
	return sm.rateLimiter.Allow(clientID)
}

func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	counter, exists := rl.requests[clientID]
	now := time.Now()
	
	if !exists || now.After(counter.ResetAt) {
		rl.requests[clientID] = &RequestCounter{
			Count: 1,
			ResetAt: now.Add(rl.window),
		}
		return true
	}
	
	if counter.Count >= rl.maxRequests {
		return false
	}
	
	counter.Count++
	return true
}

func (sm *SecurityManager) cleanupExpiredTokens() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for key, token := range sm.tokens {
			if now.After(token.ExpiresAt) {
				delete(sm.tokens, key)
			}
		}
		sm.mu.Unlock()
	}
}
