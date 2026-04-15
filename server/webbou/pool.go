package webbou

import (
	"sync"
)

// BufferPool manages reusable byte buffers to reduce allocations
type BufferPool struct {
	pool sync.Pool
}

func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

func (bp *BufferPool) Put(buf []byte) {
	bp.pool.Put(buf)
}

// FramePool manages reusable Frame objects
type FramePool struct {
	pool sync.Pool
}

func NewFramePool() *FramePool {
	return &FramePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Frame{}
			},
		},
	}
}

func (fp *FramePool) Get() *Frame {
	return fp.pool.Get().(*Frame)
}

func (fp *FramePool) Put(frame *Frame) {
	// Reset frame before returning to pool
	frame.Payload = frame.Payload[:0]
	frame.Flags = 0
	fp.pool.Put(frame)
}

// Global pools
var (
	smallBufferPool  = NewBufferPool(4096)
	mediumBufferPool = NewBufferPool(16384)
	largeBufferPool  = NewBufferPool(65536)
	framePool        = NewFramePool()
)

func GetBuffer(size int) []byte {
	switch {
	case size <= 4096:
		return smallBufferPool.Get()
	case size <= 16384:
		return mediumBufferPool.Get()
	default:
		return largeBufferPool.Get()
	}
}

func PutBuffer(buf []byte, size int) {
	switch {
	case size <= 4096:
		smallBufferPool.Put(buf)
	case size <= 16384:
		mediumBufferPool.Put(buf)
	default:
		largeBufferPool.Put(buf)
	}
}

func GetFrame() *Frame {
	return framePool.Get()
}

func PutFrame(frame *Frame) {
	framePool.Put(frame)
}
