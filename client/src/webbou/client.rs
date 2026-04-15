use super::protocol::{Frame, FrameType, FrameFlags, FrameReader};
use super::crypto::CryptoEngine;
use super::compression::decompress;
use super::compression;
use std::sync::Arc;
use tokio::sync::RwLock;
use tokio::net::TcpStream;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use std::collections::HashMap;

pub struct WebBouClient {
    server_addr: String,
    connection: Arc<RwLock<Option<Connection>>>,
    crypto: Arc<CryptoEngine>,
    stats: Arc<RwLock<ClientStats>>,
}

struct Connection {
    stream: TcpStream,
    reader: FrameReader,
    next_stream_id: u32,
    active_streams: HashMap<u32, StreamState>,
}

struct StreamState {
    reliable: bool,
    fragments: Vec<Vec<u8>>,
}

#[derive(Debug, Default)]
pub struct ClientStats {
    pub bytes_sent: u64,
    pub bytes_recv: u64,
    pub frames_sent: u64,
    pub frames_recv: u64,
    pub compression_ratio: f64,
}

impl WebBouClient {
    pub fn new(server_addr: String) -> Self {
        Self {
            server_addr,
            connection: Arc::new(RwLock::new(None)),
            crypto: Arc::new(CryptoEngine::new()),
            stats: Arc::new(RwLock::new(ClientStats::default())),
        }
    }

    pub async fn connect(&self) -> Result<(), Box<dyn std::error::Error>> {
        let stream = TcpStream::connect(&self.server_addr).await?;
        
        let connection = Connection {
            stream,
            reader: FrameReader::new(),
            next_stream_id: 1,
            active_streams: HashMap::new(),
        };

        *self.connection.write().await = Some(connection);

        // Send handshake
        self.send_handshake().await?;

        Ok(())
    }

    async fn send_handshake(&self) -> Result<(), Box<dyn std::error::Error>> {
        let handshake_data = format!("WEBBOU/1.0\nPublicKey: {:?}", self.crypto.get_public_key());
        
        let frame = Frame::new(
            FrameType::Settings,
            0,
            handshake_data.into_bytes(),
        );

        self.send_frame(frame).await?;
        Ok(())
    }

    pub async fn send(&self, data: Vec<u8>, reliable: bool, compress: bool, encrypt: bool) 
        -> Result<(), Box<dyn std::error::Error>> 
    {
        let mut conn = self.connection.write().await;
        let connection = conn.as_mut().ok_or("Not connected")?;

        let stream_id = connection.next_stream_id;
        connection.next_stream_id += 1;

        let data_len = data.len();
        let mut payload = data;

        // Compress if requested
        if compress && payload.len() > 512 {
            let compressed = compression::compress(&payload)?;
            if compressed.len() < payload.len() {
                payload = compressed;
                
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
        if compress {
            frame.set_flag(FrameFlags::COMPRESSED);
        }
        if encrypt {
            frame.set_flag(FrameFlags::ENCRYPTED);
        }

        drop(conn); // Release lock before sending
        self.send_frame(frame).await?;

        Ok(())
    }

    pub async fn recv(&self) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let mut conn = self.connection.write().await;
        let connection = conn.as_mut().ok_or("Not connected")?;

        // Read from socket
        let mut buf = vec![0u8; 4096];
        let n = connection.stream.read(&mut buf).await?;
        
        if n == 0 {
            return Err("Connection closed".into());
        }

        connection.reader.feed(&buf[..n]);

        // Try to read a frame
        if let Some(mut frame) = connection.reader.read_frame()? {
            let mut stats = self.stats.write().await;
            stats.frames_recv += 1;
            stats.bytes_recv += frame.payload.len() as u64;

            // Decrypt if needed
            if frame.has_flag(FrameFlags::ENCRYPTED) {
                frame.payload = self.crypto.decrypt(&frame.payload)?;
            }

            // Decompress if needed
            if frame.has_flag(FrameFlags::COMPRESSED) {
                frame.payload = decompress(&frame.payload)?;
            }

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
        
        let frame = Frame::new(FrameType::Ping, 0, vec![]);
        self.send_frame(frame).await?;

        // Wait for pong
        loop {
            if let Ok(data) = self.recv().await {
                if data.is_empty() {
                    break;
                }
            }
        }

        Ok(start.elapsed().as_millis())
    }

    pub async fn get_stats(&self) -> ClientStats {
        self.stats.read().await.clone()
    }

    pub async fn close(&self) -> Result<(), Box<dyn std::error::Error>> {
        let frame = Frame::new(FrameType::StreamClose, 0, vec![]);
        self.send_frame(frame).await?;

        *self.connection.write().await = None;
        Ok(())
    }
}

impl Clone for ClientStats {
    fn clone(&self) -> Self {
        Self {
            bytes_sent: self.bytes_sent,
            bytes_recv: self.bytes_recv,
            frames_sent: self.frames_sent,
            frames_recv: self.frames_recv,
            compression_ratio: self.compression_ratio,
        }
    }
}
