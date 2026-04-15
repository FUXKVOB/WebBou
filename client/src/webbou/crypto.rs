use chacha20poly1305::{
    aead::{Aead, KeyInit, OsRng},
    XChaCha20Poly1305, XNonce,
};
use x25519_dalek::{EphemeralSecret, PublicKey};

pub struct CryptoEngine {
    public_key: PublicKey,
    cipher: XChaCha20Poly1305,
}

impl CryptoEngine {
    pub fn new() -> Self {
        let private_key = EphemeralSecret::random_from_rng(OsRng);
        let public_key = PublicKey::from(&private_key);

        // Generate random key for ChaCha20-Poly1305
        let key = XChaCha20Poly1305::generate_key(&mut OsRng);
        let cipher = XChaCha20Poly1305::new(&key);

        Self {
            public_key,
            cipher,
        }
    }

    pub fn encrypt(&self, plaintext: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        use chacha20poly1305::aead::rand_core::RngCore;
        
        let mut nonce_bytes = [0u8; 24];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = XNonce::from_slice(&nonce_bytes);

        let ciphertext = self.cipher
            .encrypt(nonce, plaintext)
            .map_err(|e| format!("Encryption failed: {}", e))?;

        // Prepend nonce to ciphertext
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

        let plaintext = self.cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| format!("Decryption failed: {}", e))?;

        Ok(plaintext)
    }

    pub fn get_public_key(&self) -> Vec<u8> {
        self.public_key.as_bytes().to_vec()
    }

    pub fn derive_shared_secret(&self, peer_public_key: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if peer_public_key.len() != 32 {
            return Err("Invalid public key length".into());
        }

        // Note: In production, store private_key to perform DH exchange
        // For now, return a placeholder derived key
        Ok(vec![0u8; 32])
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encrypt_decrypt() {
        let crypto = CryptoEngine::new();
        let plaintext = b"Hello, WebBou!";

        let encrypted = crypto.encrypt(plaintext).unwrap();
        assert_ne!(encrypted, plaintext);

        let decrypted = crypto.decrypt(&encrypted).unwrap();
        assert_eq!(decrypted, plaintext);
    }
}
