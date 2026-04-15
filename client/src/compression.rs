use std::io::{Read, Write};
use zstd::{Decoder, Encoder};

pub struct Compressor {
    level: i32,
}

impl Compressor {
    pub fn new(level: i32) -> Self {
        Self { level }
    }

    pub fn compress(&self, data: &[u8]) -> Result<Vec<u8>, std::io::Error> {
        let mut encoder = Encoder::new(Vec::new(), self.level)?;
        encoder.write_all(data)?;
        encoder.finish()
    }

    pub fn decompress(&self, data: &[u8]) -> Result<Vec<u8>, std::io::Error> {
        let mut decoder = Decoder::new(data)?;
        let mut decompressed = Vec::new();
        decoder.read_to_end(&mut decompressed)?;
        Ok(decompressed)
    }

    pub fn should_compress(&self, data: &[u8]) -> bool {
        // Only compress if data is larger than 1KB
        data.len() > 1024
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_compression() {
        let compressor = Compressor::new(3);
        let data = b"Hello, World! ".repeat(100);
        
        let compressed = compressor.compress(&data).unwrap();
        assert!(compressed.len() < data.len());
        
        let decompressed = compressor.decompress(&compressed).unwrap();
        assert_eq!(data, decompressed.as_slice());
    }
}
