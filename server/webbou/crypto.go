package webbou

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

type CryptoEngine struct {
	privateKey [32]byte
	publicKey  [32]byte
	aead       cipher.AEAD
}

func NewCryptoEngine() (*CryptoEngine, error) {
	ce := &CryptoEngine{}

	// Generate Curve25519 keypair
	if _, err := rand.Read(ce.privateKey[:]); err != nil {
		return nil, err
	}

	curve25519.ScalarBaseMult(&ce.publicKey, &ce.privateKey)

	// Initialize ChaCha20-Poly1305
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	ce.aead = aead

	return ce, nil
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

func (ce *CryptoEngine) GetPublicKey() []byte {
	return ce.publicKey[:]
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
