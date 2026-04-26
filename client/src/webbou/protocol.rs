use crc32fast::Hasher;

// Frame types
pub const FRAME_DATA: u8 = 0x01;
pub const FRAME_PING: u8 = 0x02;
pub const FRAME_PONG: u8 = 0x03;
pub const FRAME_STREAM_OPEN: u8 = 0x04;
pub const FRAME_STREAM_CLOSE: u8 = 0x05;
#[allow(dead_code)]
pub const FRAME_ACK: u8 = 0x06;
#[allow(dead_code)]
pub const FRAME_RESET: u8 = 0x07;
pub const FRAME_SETTINGS: u8 = 0x08;
// New frame types for 0-RTT, Multi-Path, Flow Control
pub const FRAME_HELLO: u8 = 0x10;
pub const FRAME_HELLO_ACK: u8 = 0x11;
pub const FRAME_HELLO_DONE: u8 = 0x12;
pub const FRAME_MULTI_PATH: u8 = 0x20;
pub const FRAME_PATH_CLOSE: u8 = 0x21;
pub const FRAME_FLOW_CONTROL: u8 = 0x30;
pub const FRAME_MAX_DATA: u8 = 0x31;
pub const FRAME_BLOCKED: u8 = 0x32;
pub const FRAME_ACK2: u8 = 0x33;

// Frame flags
pub const FLAG_COMPRESSED: u8 = 0x01;
pub const FLAG_ENCRYPTED: u8 = 0x02;
pub const FLAG_RELIABLE: u8 = 0x04;
#[allow(dead_code)]
pub const FLAG_PRIORITY_HIGH: u8 = 0x08;
#[allow(dead_code)]
pub const FLAG_FRAGMENTED: u8 = 0x10;
#[allow(dead_code)]
pub const FLAG_FINAL: u8 = 0x20;
#[allow(dead_code)]
pub const FLAG_ZERO_RTT: u8 = 0x40;
#[allow(dead_code)]
pub const FLAG_MULTI_PATH: u8 = 0x80;

// Protocol constants
pub const MAGIC_BYTE: u8 = 0xB0;
pub const VERSION: u8 = 0x01;
pub const HEADER_SIZE: usize = 16;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[allow(dead_code)]
pub enum FrameType {
    Data,
    Ping,
    Pong,
    StreamOpen,
    StreamClose,
    Settings,
    Hello,
    HelloAck,
    HelloDone,
    MultiPath,
    PathClose,
    FlowControl,
    MaxData,
    Blocked,
    Ack2,
}

impl FrameType {
    pub fn to_u8(self) -> u8 {
        match self {
            FrameType::Data => FRAME_DATA,
            FrameType::Ping => FRAME_PING,
            FrameType::Pong => FRAME_PONG,
            FrameType::StreamOpen => FRAME_STREAM_OPEN,
            FrameType::StreamClose => FRAME_STREAM_CLOSE,
            FrameType::Settings => FRAME_SETTINGS,
            FrameType::Hello => FRAME_HELLO,
            FrameType::HelloAck => FRAME_HELLO_ACK,
            FrameType::HelloDone => FRAME_HELLO_DONE,
            FrameType::MultiPath => FRAME_MULTI_PATH,
            FrameType::PathClose => FRAME_PATH_CLOSE,
            FrameType::FlowControl => FRAME_FLOW_CONTROL,
            FrameType::MaxData => FRAME_MAX_DATA,
            FrameType::Blocked => FRAME_BLOCKED,
            FrameType::Ack2 => FRAME_ACK2,
        }
    }

    #[allow(dead_code)]
    pub fn from_u8(value: u8) -> Result<Self, Box<dyn std::error::Error>> {
        match value {
            FRAME_DATA => Ok(FrameType::Data),
            FRAME_PING => Ok(FrameType::Ping),
            FRAME_PONG => Ok(FrameType::Pong),
            FRAME_STREAM_OPEN => Ok(FrameType::StreamOpen),
            FRAME_STREAM_CLOSE => Ok(FrameType::StreamClose),
            FRAME_SETTINGS => Ok(FrameType::Settings),
            FRAME_HELLO => Ok(FrameType::Hello),
            FRAME_HELLO_ACK => Ok(FrameType::HelloAck),
            FRAME_HELLO_DONE => Ok(FrameType::HelloDone),
            FRAME_MULTI_PATH => Ok(FrameType::MultiPath),
            FRAME_PATH_CLOSE => Ok(FrameType::PathClose),
            FRAME_FLOW_CONTROL => Ok(FrameType::FlowControl),
            FRAME_MAX_DATA => Ok(FrameType::MaxData),
            FRAME_BLOCKED => Ok(FrameType::Blocked),
            FRAME_ACK2 => Ok(FrameType::Ack2),
            _ => Err("Unknown frame type".into()),
        }
    }
}

pub struct FrameFlags;

impl FrameFlags {
    pub const COMPRESSED: u8 = FLAG_COMPRESSED;
    pub const ENCRYPTED: u8 = FLAG_ENCRYPTED;
    pub const RELIABLE: u8 = FLAG_RELIABLE;
    #[allow(dead_code)]
    pub const ZERO_RTT: u8 = FLAG_ZERO_RTT;
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

        // Calculate CRC32 over the frame header fields plus payload.
        let mut hasher = Hasher::new();
        hasher.update(&[MAGIC_BYTE, VERSION, self.frame_type, self.flags]);
        hasher.update(&self.stream_id.to_be_bytes());
        hasher.update(&payload_len.to_be_bytes());
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

        // Verify checksum against the same header fields used during marshal.
        let mut hasher = Hasher::new();
        hasher.update(&[MAGIC_BYTE, VERSION, frame_type, flags]);
        hasher.update(&stream_id.to_be_bytes());
        hasher.update(&(payload_len as u32).to_be_bytes());
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
        Self { buffer: Vec::new() }
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

#[cfg(test)]
mod tests {
    use super::{Frame, FrameType};

    const GOLDEN_FRAME_HEX: &str = include_str!("../../../protocol/testdata/data_frame_v1.hex");

    fn decode_hex(input: &str) -> Vec<u8> {
        let trimmed = input.trim();
        let mut bytes = Vec::with_capacity(trimmed.len() / 2);

        let mut chars = trimmed.chars();
        while let (Some(high), Some(low)) = (chars.next(), chars.next()) {
            let high = high.to_digit(16).expect("valid hex") as u8;
            let low = low.to_digit(16).expect("valid hex") as u8;
            bytes.push((high << 4) | low);
        }

        bytes
    }

    #[test]
    fn marshals_to_golden_frame() {
        let frame = Frame::new(FrameType::Data, 7, b"ping".to_vec());
        assert_eq!(frame.marshal(), decode_hex(GOLDEN_FRAME_HEX));
    }

    #[test]
    fn unmarshals_golden_frame() {
        let frame = Frame::unmarshal(&decode_hex(GOLDEN_FRAME_HEX)).expect("golden frame");
        assert_eq!(frame.frame_type, FrameType::Data.to_u8());
        assert_eq!(frame.stream_id, 7);
        assert_eq!(frame.payload, b"ping");
    }
}
