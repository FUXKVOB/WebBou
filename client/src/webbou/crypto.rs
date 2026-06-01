use chacha20poly1305::{
    aead::{rand_core::RngCore, Aead, KeyInit, OsRng},
    XChaCha20Poly1305, XNonce,
};
use x25519_dalek::{EphemeralSecret, PublicKey};

pub struct CryptoEngine {
    private_key: EphemeralSecret,
    public_key: PublicKey,
    cipher: XChaCha20Poly1305,
    #[allow(dead_code)]
    pinned_cert_hash: Option<Vec<u8>>,
}

impl Default for CryptoEngine {
    fn default() -> Self {
        Self::new()
    }
}

impl CryptoEngine {
    pub fn new() -> Self {
        let private_key = EphemeralSecret::random_from_rng(OsRng);
        let public_key = PublicKey::from(&private_key);

        let key = XChaCha20Poly1305::generate_key(&mut OsRng);
        let cipher = XChaCha20Poly1305::new(&key);

        Self {
            private_key,
            public_key,
            cipher,
            pinned_cert_hash: None,
        }
    }

    pub fn enable_cert_pinning(&mut self, cert_hash: &[u8]) {
        self.pinned_cert_hash = Some(cert_hash.to_vec());
        tracing::info!("Certificate pinning enabled");
    }

    pub fn validate_cert_pinning(&self, cert_hash: &[u8]) -> Result<(), String> {
        if let Some(pinned) = &self.pinned_cert_hash {
            if cert_hash != pinned.as_slice() {
                return Err("Certificate pin mismatch - possible MITM attack".to_string());
            }
        }
        Ok(())
    }

    pub fn encrypt(&self, plaintext: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let mut nonce_bytes = [0u8; 24];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = XNonce::from_slice(&nonce_bytes);

        let ciphertext = self
            .cipher
            .encrypt(nonce, plaintext)
            .map_err(|e| format!("Encryption failed: {}", e))?;

        let mut result = nonce_bytes.to_vec();
        result.extend_from_slice(&ciphertext);

        Ok(result)
    }

    pub fn decrypt(&self, data: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if data.len() < 24 {
            return Err("Data too short".into());
        }

        let nonce = XNonce::from_slice(&data[..24]);
        let ciphertext = &data[24..];

        let plaintext = self
            .cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| format!("Decryption failed: {}", e))?;

        Ok(plaintext)
    }

    pub fn get_public_key(&self) -> Vec<u8> {
        self.public_key.as_bytes().to_vec()
    }

    pub fn derive_shared_secret(
        &self,
        peer_public_key: &[u8],
    ) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if peer_public_key.len() != 32 {
            return Err("Invalid public key length".into());
        }

        let mut key_array = [0u8; 32];
        key_array.copy_from_slice(peer_public_key);
        let peer_key = PublicKey::from(key_array);

        let private_key = EphemeralSecret::random_from_rng(OsRng);
        let shared = private_key.diffie_hellman(&peer_key);

        Ok(shared.as_bytes().to_vec())
    }
}
