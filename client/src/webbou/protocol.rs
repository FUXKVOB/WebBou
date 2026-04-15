use crc32fast::Hasher;

// Frame types
pub const FRAME_DATA: u8 = 0x01;
pub const FRAME_PING: u8 = 0x02;
pub const FRAME_PONG: u8 = 0x03;
pub const FRAME_STREAM_OPEN: u8 = 0x04;
pub const FRAME_STREAM_CLOSE: u8 = 0x05;

// Frame flags
pub const FLAG_COMPRESSED: u8 = 0x01;
pub const FLAG_ENCRYPTED: u8 = 0x02;
pub const FLAG_RELIABLE: u8 = 0x04;

// Protocol constants
pub const MAGIC_BYTE: u8 = 0xB0;
pub const VERSION: u8 = 0x01;
pub const HEADER_SIZE: usize = 16;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FrameType {
    Data,
    Ping,
    Pong,
    StreamOpen,
    StreamClose,
    Settings,
}

impl FrameType {
    pub fn to_u8(self) -> u8 {
        match self {
            FrameType::Data => FRAME_DATA,
            FrameType::Ping => FRAME_PING,
            FrameType::Pong => FRAME_PONG,
            FrameType::StreamOpen => FRAME_STREAM_OPEN,
            FrameType::StreamClose => FRAME_STREAM_CLOSE,
            FrameType::Settings => 0x06,
        }
    }

    pub fn from_u8(value: u8) -> Result<Self, Box<dyn std::error::Error>> {
        match value {
            FRAME_DATA => Ok(FrameType::Data),
            FRAME_PING => Ok(FrameType::Ping),
            FRAME_PONG => Ok(FrameType::Pong),
            FRAME_STREAM_OPEN => Ok(FrameType::StreamOpen),
            FRAME_STREAM_CLOSE => Ok(FrameType::StreamClose),
            0x06 => Ok(FrameType::Settings),
            _ => Err("Unknown frame type".into()),
        }
    }
}

pub struct FrameFlags;

impl FrameFlags {
    pub const COMPRESSED: u8 = FLAG_COMPRESSED;
    pub const ENCRYPTED: u8 = FLAG_ENCRYPTED;
    pub const RELIABLE: u8 = FLAG_RELIABLE;
}

#[derive(Debug, Clone)]
pub struct Frame {
    pub frame_type: u8,
    pub flags: u8,
    pub stream_id: u32,
    pub payload: Vec<u8>,
}

impl Frame {
    pub fn new(frame_type: FrameType, stream_id: u32, payload: Vec<u8>) -> Self {
        Self {
            frame_type: frame_type.to_u8(),
            flags: 0,
            stream_id,
            payload,
        }
    }

    pub fn set_flag(&mut self, flag: u8) {
        self.flags |= flag;
    }

    pub fn has_flag(&self, flag: u8) -> bool {
        (self.flags & flag) != 0
    }

    pub fn marshal(&self) -> Vec<u8> {
        let payload_len = self.payload.len() as u32;
        let total_len = HEADER_SIZE + self.payload.len();
        
        let mut buf = Vec::with_capacity(total_len);
        
        // Header
        buf.push(MAGIC_BYTE);
        buf.push(VERSION);
        buf.push(self.frame_type);
        buf.push(self.flags);
        buf.extend_from_slice(&self.stream_id.to_be_bytes());
        buf.extend_from_slice(&payload_len.to_be_bytes());
        
        // Calculate CRC32 of payload
        let mut hasher = Hasher::new();
        hasher.update(&self.payload);
        let checksum = hasher.finalize();
        buf.extend_from_slice(&checksum.to_be_bytes());
        
        // Payload
        buf.extend_from_slice(&self.payload);
        
        buf
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, Box<dyn std::error::Error>> {
        if data.len() < HEADER_SIZE {
            return Err("Data too short".into());
        }

        if data[0] != MAGIC_BYTE {
            return Err("Invalid magic byte".into());
        }

        if data[1] != VERSION {
            return Err("Unsupported version".into());
        }

        let frame_type = data[2];
        let flags = data[3];
        
        let stream_id = u32::from_be_bytes([data[4], data[5], data[6], data[7]]);
        let payload_len = u32::from_be_bytes([data[8], data[9], data[10], data[11]]) as usize;
        let checksum = u32::from_be_bytes([data[12], data[13], data[14], data[15]]);

        if data.len() < HEADER_SIZE + payload_len {
            return Err("Incomplete frame".into());
        }

        let payload = data[HEADER_SIZE..HEADER_SIZE + payload_len].to_vec();

        // Verify checksum
        let mut hasher = Hasher::new();
        hasher.update(&payload);
        if hasher.finalize() != checksum {
            return Err("Checksum mismatch".into());
        }

        Ok(Self {
            frame_type,
            flags,
            stream_id,
            payload,
        })
    }
}

pub struct FrameReader {
    buffer: Vec<u8>,
}

impl FrameReader {
    pub fn new() -> Self {
        Self {
            buffer: Vec::new(),
        }
    }

    pub fn feed(&mut self, data: &[u8]) {
        self.buffer.extend_from_slice(data);
    }

    pub fn read_frame(&mut self) -> Result<Option<Frame>, Box<dyn std::error::Error>> {
        if self.buffer.len() < HEADER_SIZE {
            return Ok(None);
        }

        // Check for magic byte
        if self.buffer[0] != MAGIC_BYTE {
            return Err("Invalid magic byte".into());
        }

        let payload_len = u32::from_be_bytes([
            self.buffer[8],
            self.buffer[9],
            self.buffer[10],
            self.buffer[11],
        ]) as usize;

        let frame_len = HEADER_SIZE + payload_len;

        if self.buffer.len() < frame_len {
            return Ok(None);
        }

        let frame_data = self.buffer.drain(..frame_len).collect::<Vec<u8>>();
        let frame = Frame::unmarshal(&frame_data)?;

        Ok(Some(frame))
    }
}
