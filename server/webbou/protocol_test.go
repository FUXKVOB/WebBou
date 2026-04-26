package webbou

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarshalGoldenFrame(t *testing.T) {
	frame := NewFrame(FrameData, 7, []byte("ping"))
	if got, want := encodeHex(frame.Marshal()), loadGoldenFrameHex(t); got != want {
		t.Fatalf("marshal mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestUnmarshalGoldenFrame(t *testing.T) {
	frame, err := UnmarshalFrame(decodeHex(t, loadGoldenFrameHex(t)))
	if err != nil {
		t.Fatalf("unmarshal golden frame: %v", err)
	}

	if frame.Type != FrameData {
		t.Fatalf("unexpected frame type: %x", frame.Type)
	}
	if frame.StreamID != 7 {
		t.Fatalf("unexpected stream id: %d", frame.StreamID)
	}
	if string(frame.Payload) != "ping" {
		t.Fatalf("unexpected payload: %q", string(frame.Payload))
	}
}

func loadGoldenFrameHex(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "protocol", "testdata", "data_frame_v1.hex")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden frame: %v", err)
	}

	return strings.TrimSpace(string(data))
}

func encodeHex(data []byte) string {
	const digits = "0123456789abcdef"

	buf := make([]byte, len(data)*2)
	for i, b := range data {
		buf[i*2] = digits[b>>4]
		buf[i*2+1] = digits[b&0x0f]
	}

	return string(buf)
}

func decodeHex(t *testing.T, input string) []byte {
	t.Helper()

	if len(input)%2 != 0 {
		t.Fatalf("hex string must have even length")
	}

	out := make([]byte, len(input)/2)
	for i := 0; i < len(input); i += 2 {
		hi := fromHexChar(t, input[i])
		lo := fromHexChar(t, input[i+1])
		out[i/2] = (hi << 4) | lo
	}

	return out
}

func fromHexChar(t *testing.T, ch byte) byte {
	t.Helper()

	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0'
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10
	default:
		t.Fatalf("invalid hex char: %q", ch)
		return 0
	}
}
