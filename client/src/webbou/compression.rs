use lz4_flex::{compress_prepend_size, decompress_size_prepended};

pub fn compress(data: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    if data.len() < 512 {
        return Ok(data.to_vec()); // Too small
    }

    let compressed = compress_prepend_size(data);

    // Only use if actually smaller
    if compressed.len() < data.len() {
        Ok(compressed)
    } else {
        Ok(data.to_vec())
    }
}

pub fn decompress(data: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    decompress_size_prepended(data).map_err(|e| format!("Decompression failed: {}", e).into())
}

#[allow(dead_code)]
pub fn should_compress(data: &[u8]) -> bool {
    data.len() > 512
}

#[allow(dead_code)]
pub fn estimate_compression_ratio(data: &[u8]) -> f64 {
    if data.len() < 100 {
        return 1.0;
    }

    // Sample entropy
    let sample = &data[..100.min(data.len())];
    let mut unique = std::collections::HashSet::new();

    for &byte in sample {
        unique.insert(byte);
    }

    let entropy = unique.len() as f64 / sample.len() as f64;

    // High entropy = poor compression
    if entropy > 0.8 {
        0.9
    } else {
        0.3
    }
}
