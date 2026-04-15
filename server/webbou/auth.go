package webbou

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// SessionManager manages authenticated sessions
type SessionManager struct {
	sessions map[string]*AuthSession
	mu       sync.RWMutex
	timeout  time.Duration
}

type AuthSession struct {
	ID           string
	Token        string
	PublicKey    []byte
	SharedSecret []byte
	CreatedAt    time.Time
	LastActivity time.Time
	Authenticated bool
}

func NewSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*AuthSession),
		timeout:  timeout,
	}

	go sm.cleanupExpired()
	return sm
}

func (sm *SessionManager) CreateSession(publicKey []byte) (*AuthSession, error) {
	sessionID, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	token, err := generateSecureToken(64)
	if err != nil {
		return nil, err
	}

	session := &AuthSession{
		ID:           sessionID,
		Token:        token,
		PublicKey:    publicKey,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Authenticated: false,
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	return session, nil
}

func (sm *SessionManager) GetSession(sessionID string) (*AuthSession, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	if time.Since(session.LastActivity) > sm.timeout {
		sm.DeleteSession(sessionID)
		return nil, errors.New("session expired")
	}

	return session, nil
}

func (sm *SessionManager) UpdateActivity(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.LastActivity = time.Now()
	return nil
}

func (sm *SessionManager) AuthenticateSession(sessionID string, sharedSecret []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.SharedSecret = sharedSecret
	session.Authenticated = true
	session.LastActivity = time.Now()

	return nil
}

func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()
}

func (sm *SessionManager) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, session := range sm.sessions {
			if now.Sub(session.LastActivity) > sm.timeout {
				delete(sm.sessions, id)
			}
		}
		sm.mu.Unlock()
	}
}

func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// TokenValidator validates authentication tokens
type TokenValidator struct {
	secret []byte
}

func NewTokenValidator(secret []byte) *TokenValidator {
	return &TokenValidator{secret: secret}
}

func (tv *TokenValidator) GenerateToken(sessionID string) string {
	data := append([]byte(sessionID), tv.secret...)
	hash := sha256.Sum256(data)
	return base64.URLEncoding.EncodeToString(hash[:])
}

func (tv *TokenValidator) ValidateToken(sessionID, token string) bool {
	expected := tv.GenerateToken(sessionID)
	return token == expected
}
