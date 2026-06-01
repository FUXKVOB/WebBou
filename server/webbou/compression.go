package webbou

import (
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

const (
	CompressionNone = 0
	CompressionLZ4  = 1
	CompressionZstd = 2
)

var (
	zstdEncoder *zstd.Encoder
	zstdDecoder *zstd.Decoder
)

func init() {
	var err error
	zstdEncoder, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		panic(err)
	}

	zstdDecoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(err)
	}
}

func Compress(data []byte) ([]byte, error) {
	// Auto-select compression based on size
	if len(data) < 512 {
		return data, nil // Too small to compress
	}

	if len(data) < 4096 {
		return CompressLZ4(data)
	}

	return CompressZstd(data)
}

func Decompress(data []byte) ([]byte, error) {
	// Try Zstd first
	if result, err := DecompressZstd(data); err == nil {
		return result, nil
	}

	// Fallback to LZ4
	return DecompressLZ4(data)
}

func CompressLZ4(data []byte) ([]byte, error) {
	buf := make([]byte, lz4.CompressBlockBound(len(data)))
	
	n, err := lz4.CompressBlock(data, buf, nil)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

func DecompressLZ4(data []byte) ([]byte, error) {
	buf := make([]byte, len(data)*4) // Estimate
	
	n, err := lz4.UncompressBlock(data, buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

func CompressZstd(data []byte) ([]byte, error) {
	return zstdEncoder.EncodeAll(data, nil), nil
}

func DecompressZstd(data []byte) ([]byte, error) {
	return zstdDecoder.DecodeAll(data, nil)
}

func ShouldCompress(data []byte) bool {
	return len(data) > 512
}

func EstimateCompressionRatio(data []byte) float64 {
	if len(data) < 100 {
		return 1.0
	}

	// Sample first 100 bytes
	sample := data[:100]
	unique := make(map[byte]bool)
	
	for _, b := range sample {
		unique[b] = true
	}

	entropy := float64(len(unique)) / 100.0
	
	// High entropy = poor compression
	if entropy > 0.8 {
		return 0.9 // ~10% compression
	}

	// Low entropy = good compression
	return 0.3 // ~70% compression
}
