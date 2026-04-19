package webbou

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	ZeroRTTMaxSize    = 64 * 1024
	ZeroRTTLifetime   = 24 * time.Hour
	MaxZeroRTTSessions = 1000
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

type ZeroRTTSession struct {
	ID           string
	PSK          []byte
	EarlySecret []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Used        bool
	Resumed     bool
}

type ZeroRTTManager struct {
	sessions map[string]*ZeroRTTSession
	mu       sync.RWMutex
}

func NewZeroRTTManager() *ZeroRTTManager {
	zrtt := &ZeroRTTManager{
		sessions: make(map[string]*ZeroRTTSession),
	}

	go zrtt.cleanupExpired()
	return zrtt
}

func (zrtt *ZeroRTTManager) CreatePSK(identity string, secret []byte) error {
	if len(zrtt.sessions) >= MaxZeroRTTSessions {
		zrtt.cleanupOldest()
	}

	psk := make([]byte, 32)
	if _, err := rand.Read(psk); err != nil {
		return err
	}

	h := sha256.Sum256(secret)
	earlySecret := h[:]

	session := &ZeroRTTSession{
		ID:           identity,
		PSK:          psk,
		EarlySecret:  earlySecret,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(ZeroRTTLifetime),
		Used:         false,
		Resumed:      false,
	}

	zrtt.mu.Lock()
	zrtt.sessions[identity] = session
	zrtt.mu.Unlock()

	return nil
}

func (zrtt *ZeroRTTManager) ValidatePSK(identity string, psk []byte) bool {
	zrtt.mu.RLock()
	session, exists := zrtt.sessions[identity]
	zrtt.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(session.ExpiresAt) {
		zrtt.mu.Lock()
		delete(zrtt.sessions, identity)
		zrtt.mu.Unlock()
		return false
	}

	if len(psk) != len(session.PSK) {
		return false
	}

	for i := 0; i < len(psk); i++ {
		if psk[i] != session.PSK[i] {
			return false
		}
	}

	session.Used = true
	session.Resumed = true

	return true
}

func (zrtt *ZeroRTTManager) GetEarlySecret(identity string) ([]byte, bool) {
	zrtt.mu.RLock()
	session, exists := zrtt.sessions[identity]
	zrtt.mu.RUnlock()

	if !exists || time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session.EarlySecret, !session.Used
}

func (zrtt *ZeroRTTManager) MarkUsed(identity string) {
	zrtt.mu.Lock()
	if session, exists := zrtt.sessions[identity]; exists {
		session.Used = true
	}
	zrtt.mu.Unlock()
}

func (zrtt *ZeroRTTManager) IsAvailable(identity string) bool {
	zrtt.mu.RLock()
	session, exists := zrtt.sessions[identity]
	zrtt.mu.RUnlock()

	if !exists {
		return false
	}

	return !session.Used && time.Now().Before(session.ExpiresAt)
}

func (zrtt *ZeroRTTManager) DeletePSK(identity string) {
	zrtt.mu.Lock()
	delete(zrtt.sessions, identity)
	zrtt.mu.Unlock()
}

func (zrtt *ZeroRTTManager) cleanupOldest() {
	zrtt.mu.Lock()
	defer zrtt.mu.Unlock()

	var oldestID string
	var oldestTime time.Time

	for id, session := range zrtt.sessions {
		if oldestTime.IsZero() || session.CreatedAt.Before(oldestTime) {
			oldestTime = session.CreatedAt
			oldestID = id
		}
	}

	if oldestID != "" {
		delete(zrtt.sessions, oldestID)
	}
}

func (zrtt *ZeroRTTManager) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		zrtt.mu.Lock()
		now := time.Now()
		for id, session := range zrtt.sessions {
			if now.After(session.ExpiresAt) {
				delete(zrtt.sessions, id)
			}
		}
		zrtt.mu.Unlock()
	}
}

type FlowControlManager struct {
	maxData        uint64
	maxStreamData  uint64
	availData      uint64
	availStreamData map[uint32]uint64
	mu             sync.RWMutex
}

func NewFlowControlManager() *FlowControlManager {
	return &FlowControlManager{
		maxData:        16 * 1024 * 1024,
		maxStreamData:  16 * 1024 * 1024,
		availData:      16 * 1024 * 1024,
		availStreamData: make(map[uint32]uint64),
	}
}

func (fc *FlowControlManager) InitConnection(maxData uint64) {
	fc.mu.Lock()
	fc.maxData = maxData
	fc.availData = maxData
	fc.mu.Unlock()
}

func (fc *FlowControlManager) InitStream(streamID uint32, maxData uint64) {
	fc.mu.Lock()
	fc.availStreamData[streamID] = maxData
	fc.mu.Unlock()
}

func (fc *FlowControlManager) ConsumeData(amount uint64) bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.availData >= amount {
		fc.availData -= amount
		return true
	}
	return false
}

func (fc *FlowControlManager) ConsumeStreamData(streamID uint32, amount uint64) bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	avail, exists := fc.availStreamData[streamID]
	if !exists {
		fc.availStreamData[streamID] = fc.maxStreamData - amount
		return amount <= fc.maxStreamData
	}

	if avail >= amount {
		fc.availStreamData[streamID] = avail - amount
		return true
	}
	return false
}

func (fc *FlowControlManager) UpdateMaxData(maxData uint64) {
	fc.mu.Lock()
	fc.maxData = maxData
	fc.availData = maxData
	fc.mu.Unlock()
}

func (fc *FlowControlManager) UpdateMaxStreamData(streamID uint32, maxData uint64) {
	fc.mu.Lock()
	fc.availStreamData[streamID] = maxData
	fc.mu.Unlock()
}

func (fc *FlowControlManager) IsConnectionBlocked() bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.availData == 0
}

func (fc *FlowControlManager) IsStreamBlocked(streamID uint32) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	avail, exists := fc.availStreamData[streamID]
	if !exists {
		return false
	}
	return avail == 0
}

func (fc *FlowControlManager) GetAvailableData() uint64 {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.availData
}

func (fc *FlowControlManager) GetAvailableStreamData(streamID uint32) uint64 {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.availStreamData[streamID]
}

type MultiPathManager struct {
	paths     map[uint32]*Path
	activeID  uint32
	mu       sync.RWMutex
}

type Path struct {
	ID          uint32
	LocalAddr   string
	RemoteAddr string
	Active     bool
	Latency    time.Duration
	LastActive time.Time
	RX         uint64
	TX         uint64
}

func NewMultiPathManager() *MultiPathManager {
	return &MultiPathManager{
		paths:    make(map[uint32]*Path),
		activeID: 0,
	}
}

func (mp *MultiPathManager) AddPath(localAddr, remoteAddr string) uint32 {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.activeID++
	path := &Path{
		ID:          mp.activeID,
		LocalAddr:   localAddr,
		RemoteAddr: remoteAddr,
		Active:     true,
		LastActive: time.Now(),
	}

	mp.paths[mp.activeID] = path
	return mp.activeID
}

func (mp *MultiPathManager) RemovePath(pathID uint32) {
	mp.mu.Lock()
	if path, exists := mp.paths[pathID]; exists {
		path.Active = false
	}
	mp.mu.Unlock()
}

func (mp *MultiPathManager) GetActivePath() (uint32, bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	for id, path := range mp.paths {
		if path.Active {
			return id, true
		}
	}
	return 0, false
}

func (mp *MultiPathManager) GetBestPath() (uint32, bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var bestID uint32
	var bestLatency time.Duration

	for id, path := range mp.paths {
		if path.Active && (bestID == 0 || path.Latency < bestLatency) {
			bestID = id
			bestLatency = path.Latency
		}
	}

	return bestID, bestID != 0
}

func (mp *MultiPathManager) UpdateLatency(pathID uint32, latency time.Duration) {
	mp.mu.Lock()
	if path, exists := mp.paths[pathID]; exists {
		path.Latency = latency
		path.LastActive = time.Now()
	}
	mp.mu.Unlock()
}

func (mp *MultiPathManager) GetPathStats(pathID uint32) (uint64, uint64, bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if path, exists := mp.paths[pathID]; exists {
		return path.RX, path.TX, path.Active
	}
	return 0, 0, false
}

func (mp *MultiPathManager) GetPathCount() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	count := 0
	for _, path := range mp.paths {
		if path.Active {
			count++
		}
	}
	return count
}
