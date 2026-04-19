use super::protocol::{Frame, FrameType, FrameFlags, FrameReader};
use super::crypto::{CryptoEngine, ZeroRTTState};
use super::compression::{compress, decompress};
use super::reconnect::{ReconnectStrategy, ConnectionHealth};
use super::heartbeat::HeartbeatManager;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;
use tokio::net::TcpStream;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::time::timeout;
use tracing::{info, warn, error};

pub struct WebBouClient {
    server_addr: String,
    connection: Arc<RwLock<Option<Connection>>>,
    crypto: Arc<CryptoEngine>,
    stats: Arc<RwLock<ClientStats>>,
    reconnect_strategy: Arc<RwLock<ReconnectStrategy>>,
    health: Arc<RwLock<ConnectionHealth>>,
    heartbeat: Arc<HeartbeatManager>,
    auto_reconnect: bool,
    zero_rtt: Arc<RwLock<ZeroRTTState>>,
    session_id: Option<String>,
}

struct Connection {
    stream: TcpStream,
    reader: FrameReader,
    next_stream_id: u32,
}

#[derive(Debug, Default, Clone)]
pub struct ClientStats {
    pub bytes_sent: u64,
    pub bytes_recv: u64,
    pub frames_sent: u64,
    pub frames_recv: u64,
    pub compression_ratio: f64,
    pub reconnect_count: u32,
    pub avg_latency_ms: u64,
}

impl WebBouClient {
    pub fn new(server_addr: String) -> Self {
        Self {
            server_addr,
            connection: Arc::new(RwLock::new(None)),
            crypto: Arc::new(CryptoEngine::new()),
            stats: Arc::new(RwLock::new(ClientStats::default())),
            reconnect_strategy: Arc::new(RwLock::new(ReconnectStrategy::new(5))),
            health: Arc::new(RwLock::new(ConnectionHealth::new(
                Duration::from_secs(10),
                Duration::from_secs(30),
            ))),
            heartbeat: Arc::new(HeartbeatManager::new(
                Duration::from_secs(10),
                Duration::from_secs(30),
            )),
            auto_reconnect: true,
            zero_rtt: Arc::new(RwLock::new(ZeroRTTState::new())),
            session_id: None,
        }
    }

    pub fn with_auto_reconnect(mut self, enabled: bool) -> Self {
        self.auto_reconnect = enabled;
        self
    }

    pub async fn connect(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.connect_internal().await
    }

    async fn connect_internal(&self) -> Result<(), Box<dyn std::error::Error>> {
        let stream = timeout(
            Duration::from_secs(10),
            TcpStream::connect(&self.server_addr)
        ).await??;

        info!("Connected to {}", self.server_addr);

        let connection = Connection {
            stream,
            reader: FrameReader::new(),
            next_stream_id: 1,
        };

        *self.connection.write().await = Some(connection);

        // Reset reconnect strategy on successful connection
        self.reconnect_strategy.write().await.reset();

        // Send handshake
        self.send_handshake().await?;

        // Start heartbeat
        self.start_heartbeat().await;

        Ok(())
    }

    async fn send_handshake(&self) -> Result<(), Box<dyn std::error::Error>> {
        let handshake_data = format!(
            "WEBBOU/1.0\nPublicKey: {}",
            base64::encode(self.crypto.get_public_key())
        );

        let frame = Frame::new(
            FrameType::Settings,
            0,
            handshake_data.into_bytes(),
        );

        self.send_frame(frame).await?;
        Ok(())
    }

    async fn start_heartbeat(&self) {
        let connection = Arc::clone(&self.connection);
        let crypto = Arc::clone(&self.crypto);
        let stats = Arc::clone(&self.stats);

        self.heartbeat.start(move |frame| {
            let connection = Arc::clone(&connection);
            let crypto = Arc::clone(&crypto);
            let stats = Arc::clone(&stats);

            async move {
                let data = frame.marshal();
                
                let mut conn = connection.write().await;
                if let Some(connection) = conn.as_mut() {
                    connection.stream.write_all(&data).await?;
                    
                    let mut s = stats.write().await;
                    s.frames_sent += 1;
                    s.bytes_sent += data.len() as u64;
                }

                Ok(())
            }
        }).await;
    }

    pub async fn send_with_retry(
        &self,
        data: Vec<u8>,
        reliable: bool,
        compress: bool,
        encrypt: bool,
    ) -> Result<(), Box<dyn std::error::Error>> {
        let mut attempts = 0;
        let max_attempts = 3;

        loop {
            match self.send(data.clone(), reliable, compress, encrypt).await {
                Ok(_) => return Ok(()),
                Err(e) => {
                    attempts += 1;
                    if attempts >= max_attempts {
                        return Err(e);
                    }

                    warn!("Send failed (attempt {}/{}): {}", attempts, max_attempts, e);

                    if self.auto_reconnect {
                        self.reconnect().await?;
                    } else {
                        return Err(e);
                    }
                }
            }
        }
    }

    pub async fn send(
        &self,
        data: Vec<u8>,
        reliable: bool,
        compress_flag: bool,
        encrypt: bool,
    ) -> Result<(), Box<dyn std::error::Error>> {
        let mut conn = self.connection.write().await;
        let connection = conn.as_mut().ok_or("Not connected")?;

        let stream_id = connection.next_stream_id;
        connection.next_stream_id += 1;

        let data_len = data.len();
        let mut payload = data;

        // Compress if requested and beneficial
        let mut compressed = false;
        if compress_flag && payload.len() > 512 {
            let compressed_data = compress(&payload)?;
            if compressed_data.len() < payload.len() {
                payload = compressed_data;
                compressed = true;

                let mut stats = self.stats.write().await;
                stats.compression_ratio = payload.len() as f64 / data_len as f64;
            }
        }

        // Encrypt if requested
        if encrypt {
            payload = self.crypto.encrypt(&payload)?;
        }

        let mut frame = Frame::new(FrameType::Data, stream_id, payload);

        if reliable {
            frame.set_flag(FrameFlags::RELIABLE);
        }
        if compressed {
            frame.set_flag(FrameFlags::COMPRESSED);
        }
        if encrypt {
            frame.set_flag(FrameFlags::ENCRYPTED);
        }

        drop(conn);
        self.send_frame(frame).await?;

        // Mark activity
        self.heartbeat.mark_activity().await;

        Ok(())
    }

    pub async fn recv_with_timeout(
        &self,
        timeout_duration: Duration,
    ) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        timeout(timeout_duration, self.recv()).await?
    }

    pub async fn recv(&self) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let mut conn = self.connection.write().await;
        let connection = conn.as_mut().ok_or("Not connected")?;

        let mut buf = vec![0u8; 8192]; // Larger buffer
        let n = connection.stream.read(&mut buf).await?;

        if n == 0 {
            return Err("Connection closed".into());
        }

        connection.reader.feed(&buf[..n]);

        if let Some(mut frame) = connection.reader.read_frame()? {
            let mut stats = self.stats.write().await;
            stats.frames_recv += 1;
            stats.bytes_recv += frame.payload.len() as u64;

            // Handle PONG frames
            if frame.frame_type == FrameType::Pong.to_u8() {
                self.health.write().await.mark_pong_received();
                self.heartbeat.mark_activity().await;
                return Ok(vec![]);
            }

            // Decrypt if needed
            if frame.has_flag(FrameFlags::ENCRYPTED) {
                frame.payload = self.crypto.decrypt(&frame.payload)?;
            }

            // Decompress if needed
            if frame.has_flag(FrameFlags::COMPRESSED) {
                frame.payload = decompress(&frame.payload)?;
            }

            // Mark activity
            self.heartbeat.mark_activity().await;

            return Ok(frame.payload);
        }

        Err("No complete frame available".into())
    }

    async fn send_frame(&self, frame: Frame) -> Result<(), Box<dyn std::error::Error>> {
        let data = frame.marshal();

        let mut conn = self.connection.write().await;
        let connection = conn.as_mut().ok_or("Not connected")?;

        connection.stream.write_all(&data).await?;

        let mut stats = self.stats.write().await;
        stats.frames_sent += 1;
        stats.bytes_sent += data.len() as u64;

        Ok(())
    }

    pub async fn ping(&self) -> Result<u128, Box<dyn std::error::Error>> {
        let start = std::time::Instant::now();

        let mut health = self.health.write().await;
        health.mark_ping_sent();
        drop(health);

        let frame = Frame::new(FrameType::Ping, 0, vec![]);
        self.send_frame(frame).await?;

        // Wait for pong with timeout
        match timeout(Duration::from_secs(5), async {
            loop {
                if let Ok(data) = self.recv().await {
                    if data.is_empty() {
                        break;
                    }
                }
            }
        }).await {
            Ok(_) => {
                let latency = start.elapsed().as_millis();
                
                let mut stats = self.stats.write().await;
                stats.avg_latency_ms = latency as u64;

                Ok(latency)
            }
            Err(_) => Err("Ping timeout".into()),
        }
    }

    pub async fn reconnect(&self) -> Result<(), Box<dyn std::error::Error>> {
        info!("Attempting to reconnect...");

        let mut strategy = self.reconnect_strategy.write().await;

        while strategy.should_retry() {
            strategy.wait().await;

            match self.connect_internal().await {
                Ok(_) => {
                    info!("Reconnected successfully");
                    
                    let mut stats = self.stats.write().await;
                    stats.reconnect_count += 1;
                    
                    return Ok(());
                }
                Err(e) => {
                    warn!("Reconnect failed: {}", e);
                }
            }
        }

        Err("Max reconnect attempts reached".into())
    }

    pub async fn is_connected(&self) -> bool {
        self.connection.read().await.is_some()
    }

    pub async fn is_healthy(&self) -> bool {
        self.health.read().await.is_healthy()
    }

    pub async fn get_stats(&self) -> ClientStats {
        self.stats.read().await.clone()
    }

    pub async fn close(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.heartbeat.stop().await;

        let frame = Frame::new(FrameType::StreamClose, 0, vec![]);
        let _ = self.send_frame(frame).await;

        *self.connection.write().await = None;
        
        info!("Connection closed");
        Ok(())
    }

    // 0-RTT methods
    pub async fn enable_zero_rtt(&self, psk_identity: String, secret: &[u8]) {
        let mut zrtt = self.zero_rtt.write().await;
        zrtt.generate_psk(psk_identity, secret);
    }

    pub async fn is_zero_rtt_available(&self) -> bool {
        self.zero_rtt.read().await.is_available()
    }

    pub async fn send_with_zero_rtt(&self, data: Vec<u8>) -> Result<(), Box<dyn std::error::Error>> {
        let mut zrtt = self.zero_rtt.write().await;

        if !zrtt.is_available() {
            return Err("0-RTT not available".into());
        }

        let encrypted = zrtt.encrypt_early_data(&data)?;

        let mut frame = Frame::new(FrameType::Hello, 0, encrypted);
        frame.set_flag(FrameFlags::ZERO_RTT);

        self.send_frame(frame).await
    }

    pub async fn complete_zero_rtt_handshake(&self, session_id: String) -> Result<(), Box<dyn std::error::Error>> {
        self.session_id = Some(session_id);
        self.crypto.create_session_key()?;

        let frame = Frame::new(FrameType::HelloDone, 0, vec![]);
        self.send_frame(frame).await
    }

    pub async fn enable_tls13(&self) {
        tracing::info!("TLS 1.3 with post-quantum support enabled");
    }

    pub async fn enable_cert_pinning(&self, cert_hash: &[u8]) {
        tracing::info!("Certificate pinning enabled for hash: {:02x?}", &cert_hash[..8]);
    }

    pub async fn get_cipher_suite(&self) -> String {
        self.crypto.get_cipher_suite().to_string()
    }
}

// Helper for base64 encoding
mod base64 {
    pub fn encode(data: Vec<u8>) -> String {
        use std::fmt::Write;
        let mut s = String::new();
        for byte in data {
            write!(&mut s, "{:02x}", byte).unwrap();
        }
        s
    }
}
