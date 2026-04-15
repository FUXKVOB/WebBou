package webbou

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

const (
	MagicByte   = 0xB0
	Version     = 0x01
	HeaderSize  = 16
)

// Frame types
const (
	FrameData        = 0x01
	FramePing        = 0x02
	FramePong        = 0x03
	FrameStreamOpen  = 0x04
	FrameStreamClose = 0x05
	FrameAck         = 0x06
	FrameReset       = 0x07
	FrameSettings    = 0x08
)

// Flags
const (
	FlagCompressed   = 0x01
	FlagEncrypted    = 0x02
	FlagReliable     = 0x04
	FlagPriorityHigh = 0x08
	FlagFragmented   = 0x10
	FlagFinal        = 0x20
)

type Frame struct {
	Magic      uint8
	Version    uint8
	Type       uint8
	Flags      uint8
	StreamID   uint32
	Length     uint32
	Checksum   uint32
	Payload    []byte
}

func NewFrame(frameType uint8, streamID uint32, payload []byte) *Frame {
	frame := &Frame{
		Magic:    MagicByte,
		Version:  Version,
		Type:     frameType,
		Flags:    0,
		StreamID: streamID,
		Length:   uint32(len(payload)),
		Payload:  payload,
	}
	frame.Checksum = frame.calculateChecksum()
	return frame
}

func (f *Frame) SetFlag(flag uint8) {
	f.Flags |= flag
}

func (f *Frame) HasFlag(flag uint8) bool {
	return (f.Flags & flag) != 0
}

func (f *Frame) calculateChecksum() uint32 {
	data := make([]byte, 12+len(f.Payload))
	data[0] = f.Magic
	data[1] = f.Version
	data[2] = f.Type
	data[3] = f.Flags
	binary.BigEndian.PutUint32(data[4:8], f.StreamID)
	binary.BigEndian.PutUint32(data[8:12], f.Length)
	copy(data[12:], f.Payload)
	
	return crc32.ChecksumIEEE(data)
}

func (f *Frame) Validate() error {
	if f.Magic != MagicByte {
		return errors.New("invalid magic byte")
	}
	if f.Version != Version {
		return errors.New("unsupported version")
	}
	if f.Length != uint32(len(f.Payload)) {
		return errors.New("length mismatch")
	}
	if f.Checksum != f.calculateChecksum() {
		return errors.New("checksum mismatch")
	}
	return nil
}

func (f *Frame) Marshal() []byte {
	buf := make([]byte, HeaderSize+len(f.Payload))
	
	buf[0] = f.Magic
	buf[1] = f.Version
	buf[2] = f.Type
	buf[3] = f.Flags
	binary.BigEndian.PutUint32(buf[4:8], f.StreamID)
	binary.BigEndian.PutUint32(buf[8:12], f.Length)
	binary.BigEndian.PutUint32(buf[12:16], f.Checksum)
	copy(buf[16:], f.Payload)
	
	return buf
}

func UnmarshalFrame(data []byte) (*Frame, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("data too short")
	}
	
	frame := &Frame{
		Magic:    data[0],
		Version:  data[1],
		Type:     data[2],
		Flags:    data[3],
		StreamID: binary.BigEndian.Uint32(data[4:8]),
		Length:   binary.BigEndian.Uint32(data[8:12]),
		Checksum: binary.BigEndian.Uint32(data[12:16]),
	}
	
	if len(data) < HeaderSize+int(frame.Length) {
		return nil, errors.New("incomplete frame")
	}
	
	frame.Payload = data[HeaderSize : HeaderSize+frame.Length]
	
	if err := frame.Validate(); err != nil {
		return nil, err
	}
	
	return frame, nil
}

type FrameReader struct {
	buffer []byte
	offset int
}

func NewFrameReader() *FrameReader {
	return &FrameReader{
		buffer: make([]byte, 0, 65536),
	}
}

func (fr *FrameReader) Feed(data []byte) {
	fr.buffer = append(fr.buffer, data...)
}

func (fr *FrameReader) ReadFrame() (*Frame, error) {
	if len(fr.buffer) < HeaderSize {
		return nil, nil // Need more data
	}
	
	length := binary.BigEndian.Uint32(fr.buffer[8:12])
	totalSize := HeaderSize + int(length)
	
	if len(fr.buffer) < totalSize {
		return nil, nil // Need more data
	}
	
	frame, err := UnmarshalFrame(fr.buffer[:totalSize])
	if err != nil {
		return nil, err
	}
	
	// Remove processed data
	fr.buffer = fr.buffer[totalSize:]
	
	return frame, nil
}
