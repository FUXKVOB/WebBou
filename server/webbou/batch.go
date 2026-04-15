package webbou

import (
	"sync"
	"time"
)

// FrameBatcher batches multiple frames for efficient sending
type FrameBatcher struct {
	frames    []*Frame
	mu        sync.Mutex
	maxFrames int
	maxDelay  time.Duration
	flushChan chan struct{}
	onFlush   func([]*Frame)
}

func NewFrameBatcher(maxFrames int, maxDelay time.Duration, onFlush func([]*Frame)) *FrameBatcher {
	fb := &FrameBatcher{
		frames:    make([]*Frame, 0, maxFrames),
		maxFrames: maxFrames,
		maxDelay:  maxDelay,
		flushChan: make(chan struct{}, 1),
		onFlush:   onFlush,
	}

	go fb.flushLoop()
	return fb
}

func (fb *FrameBatcher) Add(frame *Frame) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	fb.frames = append(fb.frames, frame)

	if len(fb.frames) >= fb.maxFrames {
		fb.triggerFlush()
	}
}

func (fb *FrameBatcher) triggerFlush() {
	select {
	case fb.flushChan <- struct{}{}:
	default:
	}
}

func (fb *FrameBatcher) flushLoop() {
	ticker := time.NewTicker(fb.maxDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fb.flush()
		case <-fb.flushChan:
			fb.flush()
		}
	}
}

func (fb *FrameBatcher) flush() {
	fb.mu.Lock()
	if len(fb.frames) == 0 {
		fb.mu.Unlock()
		return
	}

	frames := make([]*Frame, len(fb.frames))
	copy(frames, fb.frames)
	fb.frames = fb.frames[:0]
	fb.mu.Unlock()

	if fb.onFlush != nil {
		fb.onFlush(frames)
	}
}

func (fb *FrameBatcher) Flush() {
	fb.flush()
}
