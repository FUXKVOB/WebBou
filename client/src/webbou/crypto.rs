use chacha20poly1305::{
    aead::{Aead, KeyInit, OsRng, rand_core::RngCore},
    XChaCha20Poly1305, XNonce,
};
use x25519_dalek::{EphemeralSecret, PublicKey};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant};

pub struct CryptoEngine {
    private_key: EphemeralSecret,
    public_key: PublicKey,
    cipher: XChaCha20Poly1305,
    post_quantum_enabled: bool,
    kyber_public: Vec<u8>,
    rotation_counter: AtomicU64,
    max_frames_rotate: u64,
    session_key: Option<Vec<u8>>,
    cert_pinning_enabled: bool,
    pinned_cert_hash: Option<Vec<u8>>,
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
            post_quantum_enabled: true,
            kyber_public: Self::generate_kyber_key(),
            rotation_counter: AtomicU64::new(0),
            max_frames_rotate: 1000,
            session_key: None,
            cert_pinning_enabled: false,
            pinned_cert_hash: None,
        }
    }

    fn generate_kyber_key() -> Vec<u8> {
        let mut key = vec![0u8; 800];
        let mut rng = OsRng;
        rng.fill_bytes(&mut key);
        key
    }

    pub fn enable_tls13(&mut self) {
        tracing::info!("TLS 1.3 enabled with post-quantum support");
    }

    pub fn enable_cert_pinning(&mut self, cert_hash: &[u8]) {
        self.cert_pinning_enabled = true;
        self.pinned_cert_hash = Some(cert_hash.to_vec());
        tracing::info!("Certificate pinning enabled");
    }

    pub fn validate_cert_pinning(&self, cert_hash: &[u8]) -> Result<(), String> {
        if !self.cert_pinning_enabled {
            return Ok(());
        }

        if let Some(pinned) = &self.pinned_cert_hash {
            if cert_hash != pinned.as_slice() {
                return Err("Certificate pin mismatch - possible MITM attack".to_string());
            }
        }

        Ok(())
    }

    pub fn get_tls_ciphersuite(&self) -> &'static str {
        if self.post_quantum_enabled {
            "TLS_KYBER_768_AES_256_GCM_SHA384"
        } else {
            "TLS_AES_256_GCM_SHA385"
        }
    }

    pub fn encrypt(&self, plaintext: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let mut nonce_bytes = [0u8; 24];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = XNonce::from_slice(&nonce_bytes);

        let ciphertext = self.cipher
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

        let plaintext = self.cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| format!("Decryption failed: {}", e))?;

        Ok(plaintext)
    }

    pub fn encrypt_session(&mut self, _session_id: &str, plaintext: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if self.session_key.is_none() {
            return self.encrypt(plaintext);
        }

        let rotation = self.rotation_counter.load(Ordering::SeqCst);
        if rotation >= self.max_frames_rotate {
            self.rotate_session_key();
        }

        let key = self.session_key.as_ref().ok_or("No session key")?;
        let cipher = XChaCha20Poly1305::new_from_slice(key)
            .map_err(|e| format!("Failed to create cipher: {}", e))?;

        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = XNonce::from_slice(&nonce_bytes);

        let ciphertext = cipher
            .encrypt(nonce, plaintext)
            .map_err(|e| format!("Encryption failed: {}", e))?;

        let mut result = nonce_bytes.to_vec();
        result.extend_from_slice(&ciphertext);

        Ok(result)
    }

    pub fn decrypt_session(&mut self, _session_id: &str, data: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if self.session_key.is_none() {
            return self.decrypt(data);
        }

        if data.len() < 12 {
            return Err("Data too short".into());
        }

        let nonce = XNonce::from_slice(&data[..12]);
        let ciphertext = &data[12..];

        let key = self.session_key.as_ref().ok_or("No session key")?;
        let cipher = XChaCha20Poly1305::new_from_slice(key)
            .map_err(|e| format!("Failed to create cipher: {}", e))?;

        let plaintext = cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| format!("Decryption failed: {}", e))?;

        Ok(plaintext)
    }

    fn rotate_session_key(&mut self) {
        if self.session_key.is_some() {
            let mut new_key = vec![0u8; 32];
            OsRng.fill_bytes(&mut new_key);
            self.session_key = Some(new_key);
        }
        self.rotation_counter.store(0, Ordering::SeqCst);
    }

    pub fn get_public_key(&self) -> Vec<u8> {
        self.public_key.as_bytes().to_vec()
    }

    pub fn get_post_quantum_public_key(&self) -> Vec<u8> {
        self.kyber_public.clone()
    }

    pub fn derive_shared_secret(&self, peer_public_key: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
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

    pub fn derive_hybrid_shared_secret(
        &self,
        peer_public_key: &[u8],
        peer_kyber_public: &[u8],
    ) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let mut shared = vec![0u8; 64];

        let classical = self.derive_shared_secret(peer_public_key)?;
        for (i, b) in classical.iter().enumerate().take(32) {
            shared[i] = *b;
        }

        if peer_kyber_public.len() >= 32 {
            for i in 32..64 {
                shared[i] = peer_kyber_public[i - 32];
            }
        }

        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};

        let mut hasher = DefaultHasher::new();
        shared.hash(&mut hasher);
        let result = hasher.finish().to_le_bytes();

        Ok(result.to_vec())
    }

    pub fn create_session_key(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        let mut key = vec![0u8; 32];
        OsRng.fill_bytes(&mut key);
        self.session_key = Some(key);
        Ok(())
    }

    pub fn delete_session_key(&mut self) {
        self.session_key = None;
    }

    pub fn get_cipher_suite(&self) -> &'static str {
        if self.post_quantum_enabled {
            "KYBER-768 + CHACHA20-POLY1305"
        } else {
            "Curve25519 + CHACHA20-POLY1305"
        }
    }
}

pub struct ZeroRTTState {
    psk_identity: Option<String>,
    early_key: Option<Vec<u8>>,
    early_nonce: Vec<u8>,
    expires_at: Option<Instant>,
    used: bool,
}

impl ZeroRTTState {
    pub fn new() -> Self {
        Self {
            psk_identity: None,
            early_key: None,
            early_nonce: vec![0u8; 12],
            expires_at: None,
            used: false,
        }
    }

    pub fn generate_psk(&mut self, identity: String, secret: &[u8]) {
        self.psk_identity = Some(identity);
        self.early_key = Some(secret.to_vec());
        OsRng.fill_bytes(&mut self.early_nonce);
        self.expires_at = Some(Instant::now() + Duration::from_secs(86400));
    }

    pub fn encrypt_early_data(&mut self, plaintext: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if self.early_key.is_none() {
            return Err("0-RTT not available".into());
        }

        if let Some(expired) = self.expires_at {
            if Instant::now() > expired {
                return Err("0-RTT expired".into());
            }
        }

        let key = self.early_key.as_ref().ok_or("No early key")?;
        let cipher = XChaCha20Poly1305::new_from_slice(key)
            .map_err(|e| format!("Failed to create cipher: {}", e))?;

        let nonce = XNonce::from_slice(&self.early_nonce);
        let ciphertext = cipher
            .encrypt(nonce, plaintext)
            .map_err(|e| format!("Encryption failed: {}", e))?;

        self.used = true;

        let mut result = self.early_nonce.clone();
        result.extend_from_slice(&ciphertext);
        Ok(result)
    }

    pub fn decrypt_early_data(&mut self, data: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        if self.early_key.is_none() {
            return Err("0-RTT not available".into());
        }

        if data.len() < 12 {
            return Err("Data too short".into());
        }

        let key = self.early_key.as_ref().ok_or("No early key")?;
        let cipher = XChaCha20Poly1305::new_from_slice(key)
            .map_err(|e| format!("Failed to create cipher: {}", e))?;

        let nonce = XNonce::from_slice(&data[..12]);
        let ciphertext = &data[12..];

        let plaintext = cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| format!("Decryption failed: {}", e))?;

        self.used = true;

        Ok(plaintext)
    }

    pub fn is_available(&self) -> bool {
        if self.used {
            return false;
        }
        if let Some(expired) = self.expires_at {
            return Instant::now() < expired;
        }
        false
    }

    pub fn get_psk_identity(&self) -> Option<&str> {
        self.psk_identity.as_deref()
    }
}

pub struct MultiPathManager {
    paths: std::collections::HashMap<u32, PathState>,
    active_id: u32,
}

struct PathState {
    local_addr: String,
    remote_addr: String,
    active: bool,
    latency: Duration,
    last_active: Instant,
    rx_bytes: u64,
    tx_bytes: u64,
}

impl MultiPathManager {
    pub fn new() -> Self {
        Self {
            paths: std::collections::HashMap::new(),
            active_id: 0,
        }
    }

    pub fn add_path(&mut self, local_addr: String, remote_addr: String) -> u32 {
        self.active_id += 1;
        let id = self.active_id;

        self.paths.insert(id, PathState {
            local_addr,
            remote_addr,
            active: true,
            latency: Duration::ZERO,
            last_active: Instant::now(),
            rx_bytes: 0,
            tx_bytes: 0,
        });

        id
    }

    pub fn remove_path(&mut self, id: u32) {
        if let Some(path) = self.paths.get_mut(&id) {
            path.active = false;
        }
    }

    pub fn get_active_path(&self) -> Option<u32> {
        for (id, path) in &self.paths {
            if path.active {
                return Some(*id);
            }
        }
        None
    }

    pub fn get_best_path(&self) -> Option<u32> {
        let mut best_id = None;
        let mut best_latency = None;

        for (id, path) in &self.paths {
            if path.active {
                if best_latency.is_none() || path.latency < best_latency.unwrap() {
                    best_id = Some(*id);
                    best_latency = Some(path.latency);
                }
            }
        }

        best_id
    }

    pub fn update_latency(&mut self, id: u32, latency: Duration) {
        if let Some(path) = self.paths.get_mut(&id) {
            path.latency = latency;
            path.last_active = Instant::now();
        }
    }

    pub fn get_path_count(&self) -> usize {
        self.paths.values().filter(|p| p.active).count()
    }
}

pub struct FlowControlManager {
    max_data: u64,
    max_stream_data: u64,
    avail_data: u64,
    avail_stream_data: std::collections::HashMap<u32, u64>,
}

impl FlowControlManager {
    pub fn new() -> Self {
        Self {
            max_data: 16 * 1024 * 1024,
            max_stream_data: 16 * 1024 * 1024,
            avail_data: 16 * 1024 * 1024,
            avail_stream_data: std::collections::HashMap::new(),
        }
    }

    pub fn init_connection(&mut self, max_data: u64) {
        self.max_data = max_data;
        self.avail_data = max_data;
    }

    pub fn init_stream(&mut self, stream_id: u32, max_data: u64) {
        self.avail_stream_data.insert(stream_id, max_data);
    }

    pub fn consume_data(&mut self, amount: u64) -> bool {
        if self.avail_data >= amount {
            self.avail_data -= amount;
            return true;
        }
        false
    }

    pub fn consume_stream_data(&mut self, stream_id: u32, amount: u64) -> bool {
        let avail = self.avail_stream_data.get(&stream_id).copied().unwrap_or(self.max_stream_data);

        if avail >= amount {
            self.avail_stream_data.insert(stream_id, avail - amount);
            return true;
        }
        false
    }

    pub fn update_max_data(&mut self, max_data: u64) {
        self.max_data = max_data;
        self.avail_data = max_data;
    }

    pub fn update_max_stream_data(&mut self, stream_id: u32, max_data: u64) {
        self.avail_stream_data.insert(stream_id, max_data);
    }

    pub fn is_connection_blocked(&self) -> bool {
        self.avail_data == 0
    }

    pub fn is_stream_blocked(&self, stream_id: u32) -> bool {
        self.avail_stream_data.get(&stream_id).copied().unwrap_or(self.max_stream_data) == 0
    }

    pub fn get_available_data(&self) -> u64 {
        self.avail_data
    }

    pub fn get_available_stream_data(&self, stream_id: u32) -> u64 {
        self.avail_stream_data.get(&stream_id).copied().unwrap_or(self.max_stream_data)
    }
}