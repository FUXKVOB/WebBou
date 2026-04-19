package webbou

import (
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	_ "unsafe"
)

type BufferPool struct {
	pool sync.Pool
	size int
}

func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
		size: size,
	}
}

func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

func (bp *BufferPool) Put(buf []byte) {
	bp.pool.Put(buf[:cap(buf)])
}

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
	frame.Payload = frame.Payload[:0]
	frame.Flags = 0
	fp.pool.Put(frame)
}

var (
	smallBufferPool  = NewBufferPool(4096)
	mediumBufferPool = NewBufferPool(16384)
	largeBufferPool = NewBufferPool(65536)
	framePool      = NewFramePool()
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

type SpinLock struct {
	locked int32
}

func (sl *SpinLock) Lock() {
	for !atomic.CompareAndSwapInt32(&sl.locked, 0, 1) {
		runtime.Gosched()
	}
}

func (sl *SpinLock) Unlock() {
	atomic.StoreInt32(&sl.locked, 0)
}

type BatchSender struct {
	queue    chan *writeRequest
	workers int
	pool    *sync.Pool
}

type writeRequest struct {
	data []byte
	done  chan error
}

func NewBatchSender(workers int) *BatchSender {
	bs := &BatchSender{
		queue:    make(chan *writeRequest, 10000),
		workers: workers,
		pool: &sync.Pool{
			New: func() interface{} {
				return &writeRequest{
					done: make(chan error, 1),
				}
			},
		},
	}

	for i := 0; i < workers; i++ {
		go bs.worker(i)
	}

	return bs
}

func (bs *BatchSender) worker(_ int) {
	var pending []*writeRequest
	ticker := time.NewTicker(100 * time.Microsecond)
	defer ticker.Stop()

	for {
		select {
		case req := <-bs.queue:
			pending = append(pending, req)
			if len(pending) >= 32 {
				bs.flush(pending)
				pending = nil
			}
		case <-ticker.C:
			if len(pending) > 0 {
				bs.flush(pending)
				pending = nil
			}
		}
	}
}

func (bs *BatchSender) flush(pending []*writeRequest) {
	if len(pending) == 0 {
		return
	}

	total := 0
	for _, req := range pending {
		total += len(req.data)
	}

	buf := make([]byte, total)
	offset := 0
	for _, req := range pending {
		n := copy(buf[offset:], req.data)
		offset += n
	}

	_ = buf
	var writeErr error
	for _, req := range pending {
		select {
		case req.done <- writeErr:
		default:
		}
	}
}

func (bs *BatchSender) Write(data []byte) error {
	req := bs.pool.Get().(*writeRequest)
	defer bs.pool.Put(req)

	req.data = data
	select {
	case bs.queue <- req:
		return <-req.done
	default:
		return &serverError{msg: "server busy, try again later"}
	}
}

type serverError struct {
	msg string
}

func (e *serverError) Error() string {
	return e.msg
}

type BackPressureMonitor struct {
	thresholdLow  float64
	thresholdHigh float64
	highWaterMark uint64
	lowWaterMark  uint64
	enabled      atomic.Bool
	paused       atomic.Bool
}

func NewBackPressureMonitor(low, high float64) *BackPressureMonitor {
	defaultLimit := uint64(10000)
	return &BackPressureMonitor{
		thresholdLow:  low,
		thresholdHigh: high,
		highWaterMark: uint64(float64(defaultLimit) * high),
		lowWaterMark:  uint64(float64(defaultLimit) * low),
		enabled:      atomic.Bool{},
	}
}

func (bpm *BackPressureMonitor) Check() bool {
	if !bpm.enabled.Load() {
		return true
	}

	current := atomic.LoadUint64(&bpm.highWaterMark)
	return current < bpm.highWaterMark
}

func (bpm *BackPressureMonitor) IsPaused() bool {
	return bpm.paused.Load()
}

func (bpm *BackPressureMonitor) Pause() {
	bpm.paused.Store(true)
}

func (bpm *BackPressureMonitor) Resume() {
	bpm.paused.Store(false)
}

type DDOSProtector struct {
	maxRequestsPerWindow int
	windowSize        time.Duration
	maxConnections int
	blockDuration  time.Duration

	mu            sync.RWMutex
	requestCounts map[string]*requestCounter
	blockedIPs   map[string]time.Time
	limiter      *RateLimiter
}

type requestCounter struct {
	count        int
	windowStart time.Time
}

func NewDDOSProtector(maxReq, maxConn int, windowSec int) *DDOSProtector {
	ddos := &DDOSProtector{
		maxRequestsPerWindow: maxReq,
		windowSize:         time.Duration(windowSec) * time.Second,
		maxConnections:    maxConn,

		requestCounts: make(map[string]*requestCounter),
		blockedIPs:   make(map[string]time.Time),
		limiter:      NewRateLimiter(float64(maxReq), float64(maxReq)/float64(windowSec)),
	}

	go ddos.cleanup()
	return ddos
}

func (ddos *DDOSProtector) Allow(ip string) bool {
	ddos.mu.RLock()
	if blocked, exists := ddos.blockedIPs[ip]; exists {
		if time.Now().Before(blocked) {
			ddos.mu.RUnlock()
			return false
		}
		delete(ddos.blockedIPs, ip)
	}
	ddos.mu.RUnlock()

	if !ddos.limiter.Allow() {
		ddos.BlockIP(ip, 5*time.Minute)
		return false
	}

	ddos.mu.Lock()
	counter, exists := ddos.requestCounts[ip]
	if !exists {
		counter = &requestCounter{}
		ddos.requestCounts[ip] = counter
	}

	if time.Since(counter.windowStart) > ddos.windowSize {
		counter.count = 0
		counter.windowStart = time.Now()
	}

	counter.count++
	if counter.count > ddos.maxRequestsPerWindow {
		ddos.BlockIP(ip, 10*time.Minute)
		ddos.mu.Unlock()
		return false
	}
	ddos.mu.Unlock()

	return true
}

func (ddos *DDOSProtector) BlockIP(ip string, duration time.Duration) {
	ddos.mu.Lock()
	ddos.blockedIPs[ip] = time.Now().Add(duration)
	ddos.mu.Unlock()

	log.Printf("IP blocked: %s for %v", ip, duration)
}

func (ddos *DDOSProtector) UnblockIP(ip string) {
	ddos.mu.Lock()
	delete(ddos.blockedIPs, ip)
	delete(ddos.requestCounts, ip)
	ddos.mu.Unlock()
}

func (ddos *DDOSProtector) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ddos.mu.Lock()
		now := time.Now()
		for ip, blocked := range ddos.blockedIPs {
			if now.After(blocked) {
				delete(ddos.blockedIPs, ip)
			}
		}
		for ip, counter := range ddos.requestCounts {
			if now.Sub(counter.windowStart) > 5*ddos.windowSize {
				delete(ddos.requestCounts, ip)
			}
		}
		ddos.mu.Unlock()
	}
}

func (ddos *DDOSProtector) IsBlocked(ip string) bool {
	ddos.mu.RLock()
	defer ddos.mu.RUnlock()

	if blocked, exists := ddos.blockedIPs[ip]; exists {
		return time.Now().Before(blocked)
	}
	return false
}

type IPReputationManager struct {
	mu           sync.RWMutex
	reputations  map[string]*reputation
	badThreshold float64
}

type reputation struct {
	score        float64
	badRequests  int
	goodRequests int
	lastUpdate  time.Time
}

func NewIPReputationManager() *IPReputationManager {
	return &IPReputationManager{
		reputations:  make(map[string]*reputation),
		badThreshold: -10.0,
	}
}

func (irm *IPReputationManager) RecordSuccess(ip string) {
	irm.mu.Lock()
	defer irm.mu.Unlock()

	rep, exists := irm.reputations[ip]
	if !exists {
		rep = &reputation{lastUpdate: time.Now()}
		irm.reputations[ip] = rep
	}

	rep.score += 1.0
	rep.goodRequests++
	rep.lastUpdate = time.Now()
}

func (irm *IPReputationManager) RecordFailure(ip string) {
	irm.mu.Lock()
	defer irm.mu.Unlock()

	rep, exists := irm.reputations[ip]
	if !exists {
		rep = &reputation{lastUpdate: time.Now()}
		irm.reputations[ip] = rep
	}

	rep.score -= 2.0
	rep.badRequests++
	rep.lastUpdate = time.Now()
}

func (irm *IPReputationManager) GetScore(ip string) float64 {
	irm.mu.RLock()
	defer irm.mu.RUnlock()

	if rep, exists := irm.reputations[ip]; exists {
		return rep.score
	}
	return 0.0
}

func (irm *IPReputationManager) IsBad(ip string) bool {
	irm.mu.RLock()
	defer irm.mu.RUnlock()

	if rep, exists := irm.reputations[ip]; exists {
		return rep.score < irm.badThreshold
	}
	return false
}

func (irm *IPReputationManager) MarkAsBad(ip string) {
	irm.mu.Lock()
	defer irm.mu.Unlock()

	rep, exists := irm.reputations[ip]
	if !exists {
		rep = &reputation{}
		irm.reputations[ip] = rep
	}
	rep.score = -50.0
}

func (irm *IPReputationManager) Reset(ip string) {
	irm.mu.Lock()
	delete(irm.reputations, ip)
	irm.mu.Unlock()
}