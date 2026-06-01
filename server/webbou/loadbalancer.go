package webbou

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Backend struct {
	Addr       string
	Weight    int
	MaxConns   int
	ActiveConns atomic.Int64
	FailedReqs atomic.Int64
	LastError  time.Time
	Latency   atomic.Int64
	Healthy   atomic.Bool
}

type LoadBalancer interface {
	Select() *Backend
	Register(*Backend)
	Unregister(string)
	RecordSuccess(*Backend, time.Duration)
	RecordFailure(*Backend, error)
}

type RoundRobin struct {
	mu       sync.RWMutex
	backends []*Backend
	index    uint32
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		backends: make([]*Backend, 0),
	}
}

func (rr *RoundRobin) Select() *Backend {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if len(rr.backends) == 0 {
		return nil
	}

	index := atomic.AddUint32(&rr.index, 1)
	backend := rr.backends[(index-1)%uint32(len(rr.backends))]

	if !backend.Healthy.Load() {
		return nil
	}

	return backend
}

func (rr *RoundRobin) Register(backend *Backend) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	for _, b := range rr.backends {
		if b.Addr == backend.Addr {
			return
		}
	}

	backend.Healthy.Store(true)
	rr.backends = append(rr.backends, backend)
}

func (rr *RoundRobin) Unregister(addr string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	for i, b := range rr.backends {
		if b.Addr == addr {
			rr.backends = append(rr.backends[:i], rr.backends[i+1:]...)
			return
		}
	}
}

func (rr *RoundRobin) RecordSuccess(backend *Backend, latency time.Duration) {
	backend.FailedReqs.Store(0)
	backend.Latency.Store(int64(latency))
	backend.Healthy.Store(true)
}

func (rr *RoundRobin) RecordFailure(backend *Backend, err error) {
	backend.FailedReqs.Add(1)
	backend.LastError = time.Now()

	if backend.FailedReqs.Load() > 5 {
		backend.Healthy.Store(false)
	}
}

type LeastConnections struct {
	mu       sync.RWMutex
	backends []*Backend
}

func NewLeastConnections() *LeastConnections {
	return &LeastConnections{
		backends: make([]*Backend, 0),
	}
}

func (lc *LeastConnections) Select() *Backend {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if len(lc.backends) == 0 {
		return nil
	}

	var selected *Backend
	var minConns int64 = ^int64(0)

	for _, b := range lc.backends {
		if b.Healthy.Load() && b.ActiveConns.Load() < int64(b.MaxConns) {
			conns := b.ActiveConns.Load()
			if conns < minConns {
				minConns = conns
				selected = b
			}
		}
	}

	return selected
}

func (lc *LeastConnections) Register(backend *Backend) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	for _, b := range lc.backends {
		if b.Addr == backend.Addr {
			return
		}
	}

	backend.Healthy.Store(true)
	lc.backends = append(lc.backends, backend)
}

func (lc *LeastConnections) Unregister(addr string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	for i, b := range lc.backends {
		if b.Addr == addr {
			lc.backends = append(lc.backends[:i], lc.backends[i+1:]...)
			return
		}
	}
}

func (lc *LeastConnections) RecordSuccess(backend *Backend, latency time.Duration) {
	backend.FailedReqs.Store(0)
	backend.Latency.Store(int64(latency))
	backend.Healthy.Store(true)
}

func (lc *LeastConnections) RecordFailure(backend *Backend, err error) {
	backend.FailedReqs.Add(1)
	backend.LastError = time.Now()

	if backend.FailedReqs.Load() > 5 {
		backend.Healthy.Store(false)
	}
}

func (b *Backend) Dial(ctx context.Context) (net.Conn, error) {
	return net.DialTimeout("tcp", b.Addr, time.Duration(b.Latency.Load()))
}

type DiscoveryBackend struct {
	mu       sync.RWMutex
	addrs    []string
	interval time.Duration
	lb      LoadBalancer
}

func NewDiscoveryBackend(addrs []string, interval time.Duration, lb LoadBalancer) *DiscoveryBackend {
	db := &DiscoveryBackend{
		addrs:    addrs,
		interval: interval,
		lb:      lb,
	}

	for _, addr := range addrs {
		backend := &Backend{
			Addr:   addr,
			Weight: 100,
		}
		lb.Register(backend)
	}

	return db
}

func (db *DiscoveryBackend) Start() {
	if db.interval == 0 {
		db.interval = 30 * time.Second
	}

	go func() {
		ticker := time.NewTicker(db.interval)
		defer ticker.Stop()

		for range ticker.C {
			db.refresh()
		}
	}()
}

func (db *DiscoveryBackend) refresh() {
	db.mu.Lock()
	defer db.mu.Unlock()
}

type ProxyProtocol struct {
	mu      sync.RWMutex
	conn   net.Conn
	srcIP  string
	srcPort int
	dstIP  string
	dstPort int
}

func ParseProxyProtocol(data []byte) (*ProxyProtocol, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("data too short")
	}

	if string(data[:5]) != "PROXY" {
		return nil, fmt.Errorf("not proxy protocol")
	}

	pp := &ProxyProtocol{}
	_ = pp

	return pp, nil
}

var ErrInvalidTransition = fmt.Errorf("invalid state transition")