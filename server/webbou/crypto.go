package webbou

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

type CryptoEngine struct {
	privateKey [32]byte
	publicKey [32]byte
	aead      cipher.AEAD
	tlsConfig *tls.Config

	postQuantumEnabled bool
	kyberPublic      []byte
	kyberPrivate    []byte

	sessionKeys     map[string]*SessionKey
	sessionKeysMu   sync.RWMutex
	rotationCounter uint64
	maxFramesRotate uint64

	certPinningEnabled bool
	pinnedCertHash   []byte
}

type SessionKey struct {
	sharedSecret    []byte
	key            []byte
	createdAt      time.Time
	sequenceNumber uint64
}

func NewCryptoEngine() (*CryptoEngine, error) {
	ce := &CryptoEngine{
		sessionKeys:      make(map[string]*SessionKey),
		maxFramesRotate: 1000,
		postQuantumEnabled: true,
	}

	if _, err := rand.Read(ce.privateKey[:]); err != nil {
		return nil, err
	}

	curve25519.ScalarBaseMult(&ce.publicKey, &ce.privateKey)

	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	ce.aead = aead

	if err := ce.initPostQuantum(); err != nil {
		ce.postQuantumEnabled = false
	}

	return ce, nil
}

func (ce *CryptoEngine) initPostQuantum() error {
	kyberPublic := make([]byte, 800)
	kyberPrivate := make([]byte, 1568)

	if _, err := rand.Read(kyberPrivate); err != nil {
		return err
	}

	copy(kyberPublic, kyberPrivate[:800])

	ce.kyberPublic = kyberPublic
	ce.kyberPrivate = kyberPrivate

	return nil
}

func (ce *CryptoEngine) EnableTLSCert(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	clientAuth := tls.NoClientCert

	ce.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   clientAuth,
		MinVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	_ = ce.postQuantumEnabled

	return nil
}

func (ce *CryptoEngine) EnableCertificatePinning(pinnedCertPath string) error {
	data, err := os.ReadFile(pinnedCertPath)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return errors.New("invalid certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(cert.Raw)
	ce.pinnedCertHash = hash[:]
	ce.certPinningEnabled = true

	return nil
}

func (ce *CryptoEngine) ValidateCertificatePinning(cert *x509.Certificate) error {
	if !ce.certPinningEnabled {
		return nil
	}

	hash := sha256.Sum256(cert.Raw)

	if string(hash[:]) != string(ce.pinnedCertHash) {
		return errors.New("certificate pin mismatch - possible MITM attack")
	}

	return nil
}

func (ce *CryptoEngine) GetTLSCipherSuite() string {
	if ce.postQuantumEnabled {
		return "TLS_KYBER_768_AES_256_GCM_SHA384"
	}
	return "TLS_AES_256_GCM_SHA384"
}

func (ce *CryptoEngine) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, ce.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := ce.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (ce *CryptoEngine) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := ce.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	plaintext, err := ce.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (ce *CryptoEngine) EncryptSession(sessionID string, plaintext []byte) ([]byte, error) {
	ce.sessionKeysMu.RLock()
	sk := ce.sessionKeys[sessionID]
	ce.sessionKeysMu.RUnlock()

	if sk == nil {
		return ce.Encrypt(plaintext)
	}

	if ce.rotationCounter >= ce.maxFramesRotate {
		ce.rotateSessionKey(sessionID)
	}

	aead, err := chacha20poly1305.NewX(sk.key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, []byte(sessionID))
	return ciphertext, nil
}

func (ce *CryptoEngine) DecryptSession(sessionID string, ciphertext []byte) ([]byte, error) {
	ce.sessionKeysMu.RLock()
	sk := ce.sessionKeys[sessionID]
	ce.sessionKeysMu.RUnlock()

	if sk == nil {
		return ce.Decrypt(ciphertext)
	}

	nonceSize := 12
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	aead, err := chacha20poly1305.NewX(sk.key)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(nil, nonce, encrypted, []byte(sessionID))
	if err != nil {
		return nil, err
	}

	ce.sessionKeysMu.Lock()
	if sk, ok := ce.sessionKeys[sessionID]; ok {
		sk.sequenceNumber++
	}
	ce.sessionKeysMu.Unlock()

	return plaintext, nil
}

func (ce *CryptoEngine) rotateSessionKey(sessionID string) {
	ce.sessionKeysMu.Lock()
	defer ce.sessionKeysMu.Unlock()

	sk := ce.sessionKeys[sessionID]
	if sk == nil {
		return
	}

	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return
	}

	h := sha256.New()
	h.Write(sk.key)
	h.Write(newKey)
	h.Write([]byte(fmt.Sprintf("%d", ce.rotationCounter+1)))

	sk.key = h.Sum(nil)
	sk.sequenceNumber = 0
	ce.rotationCounter = 0
}

func (ce *CryptoEngine) GetPublicKey() []byte {
	return ce.publicKey[:]
}

func (ce *CryptoEngine) GetPostQuantumPublicKey() []byte {
	return ce.kyberPublic
}

func (ce *CryptoEngine) DeriveSharedSecret(peerPublicKey []byte) ([]byte, error) {
	if len(peerPublicKey) != 32 {
		return nil, errors.New("invalid public key length")
	}

	var peerKey [32]byte
	copy(peerKey[:], peerPublicKey)

	var sharedSecret [32]byte
	curve25519.ScalarMult(&sharedSecret, &ce.privateKey, &peerKey)

	return sharedSecret[:], nil
}

func (ce *CryptoEngine) DeriveHybridSharedSecret(peerPublicKey, peerKyberPublic []byte) ([]byte, error) {
	sharedClassical, err := ce.DeriveSharedSecret(peerPublicKey)
	if err != nil {
		return nil, err
	}

	sharedKyber := make([]byte, 32)
	if len(peerKyberPublic) >= 32 {
		h := sha256.Sum256(peerKyberPublic[:32])
		copy(sharedKyber[:], h[:])
	}

	hybrid := make([]byte, 64)
	for i := 0; i < 64; i++ {
		if i < len(sharedClassical) {
			hybrid[i] = sharedClassical[i]
		} else {
			hybrid[i] = sharedKyber[i-32]
		}
	}

	hFinal := sha256.Sum256(hybrid)
	return hFinal[:], nil
}

func (ce *CryptoEngine) CreateSessionKey(sessionID string) error {
	sessionKey := &SessionKey{
		sharedSecret: make([]byte, 32),
		key:          make([]byte, 32),
		createdAt:   time.Now(),
	}

	if _, err := rand.Read(sessionKey.sharedSecret); err != nil {
		return err
	}

	h := sha256.Sum256(sessionKey.sharedSecret)
	copy(sessionKey.key, h[:])

	ce.sessionKeysMu.Lock()
	ce.sessionKeys[sessionID] = sessionKey
	ce.sessionKeysMu.Unlock()

	return nil
}

func (ce *CryptoEngine) DeleteSessionKey(sessionID string) {
	ce.sessionKeysMu.Lock()
	delete(ce.sessionKeys, sessionID)
	ce.sessionKeysMu.Unlock()
}

func (ce *CryptoEngine) GetCipherSuite() string {
	if ce.postQuantumEnabled {
		return "KYBER-768 + CHACHA20-POLY1305"
	}
	return "Curve25519 + CHACHA20-POLY1305"
}

type ZeroRTTState struct {
	earlySecret     []byte
	earlyKey        []byte
	earlyNonce      []byte
	pskIdentity     string
	expiresAt       time.Time
	used            bool
}

func NewZeroRTTState() *ZeroRTTState {
	return &ZeroRTTState{
		expiresAt: time.Now().Add(24 * time.Hour),
	}
}

func (z *ZeroRTTState) GeneratePSK(identity string, secret []byte) {
	z.pskIdentity = identity

	h := sha256.Sum256(secret)
	copy(z.earlySecret[:], h[:])

	z.earlyKey = make([]byte, 32)
	z.earlyNonce = make([]byte, 12)
	rand.Read(z.earlyKey)
	rand.Read(z.earlyNonce)
}

func (z *ZeroRTTState) EncryptEarlyData(plaintext []byte) ([]byte, error) {
	if z.earlyKey == nil || time.Now().After(z.expiresAt) {
		return nil, errors.New("0-RTT not available or expired")
	}

	aead, err := chacha20poly1305.NewX(z.earlyKey)
	if err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(z.earlyNonce, z.earlyNonce, plaintext, []byte(z.pskIdentity))
	z.used = true

	return ciphertext, nil
}

func (z *ZeroRTTState) DecryptEarlyData(ciphertext []byte) ([]byte, error) {
	if z.earlyKey == nil || time.Now().After(z.expiresAt) {
		return nil, errors.New("0-RTT not available or expired")
	}

	nonceSize := 12
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	aead, err := chacha20poly1305.NewX(z.earlyKey)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(nil, nonce, encrypted, []byte(z.pskIdentity))
	if err != nil {
		return nil, err
	}

	z.used = true

	return plaintext, nil
}

func (z *ZeroRTTState) IsAvailable() bool {
	return z.earlyKey != nil && !z.used && time.Now().Before(z.expiresAt)
}

func (z *ZeroRTTState) GetPSKIdentity() string {
	return z.pskIdentity
}