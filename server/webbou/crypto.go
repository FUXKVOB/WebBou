package webbou

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

type CryptoEngine struct {
	privateKey [32]byte
	publicKey  [32]byte
	aead       cipher.AEAD
	tlsConfig  *tls.Config

	certPinningEnabled bool
	pinnedCertHash     []byte
}

func NewCryptoEngine() (*CryptoEngine, error) {
	ce := &CryptoEngine{}

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

	return ce, nil
}

func (ce *CryptoEngine) EnableTLSCert(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	ce.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

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
