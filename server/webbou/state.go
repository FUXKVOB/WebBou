package webbou

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ConnectionState int

const (
	StateClosed ConnectionState = iota
	StateConnecting
	StateConnected
	StateAuth
	StateReady
	StateClosing
	StateDraining
)

func (s ConnectionState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	case StateAuth:
		return "AUTH"
	case StateReady:
		return "READY"
	case StateClosing:
		return "CLOSING"
	case StateDraining:
		return "DRAINING"
	default:
		return "UNKNOWN"
	}
}

type StateMachine struct {
	mu         sync.RWMutex
	state      ConnectionState
	reason    string
	lastChange time.Time
	transitions []StateTransition
	onChange  map[ConnectionState]func(ConnectionState) error
}

type StateTransition struct {
	From     ConnectionState
	To       ConnectionState
	Time     time.Time
	Duration time.Duration
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		state:      StateClosed,
		lastChange: time.Now(),
		onChange:  make(map[ConnectionState]func(ConnectionState) error),
	}
}

func (sm *StateMachine) Current() ConnectionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

func (sm *StateMachine) SetState(newState ConnectionState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	oldState := sm.state

	if !sm.isValidTransition(oldState, newState) {
		return fmt.Errorf("invalid transition from %s to %s", oldState.String(), newState.String())
	}

	if fn, exists := sm.onChange[newState]; exists {
		if err := fn(newState); err != nil {
			return err
		}
	}

	sm.state = newState
	sm.lastChange = time.Now()
	sm.transitions = append(sm.transitions, StateTransition{
		From:     oldState,
		To:       newState,
		Time:     time.Now(),
		Duration: time.Since(sm.lastChange),
	})

	return nil
}

func (sm *StateMachine) isValidTransition(from, to ConnectionState) bool {
	validTransitions := map[ConnectionState][]ConnectionState{
		StateClosed:     {StateConnecting},
		StateConnecting: {StateConnected, StateClosed},
		StateConnected: {StateAuth, StateClosed},
		StateAuth:     {StateReady, StateClosed},
		StateReady:     {StateClosing, StateClosed},
		StateClosing:   {StateDraining, StateClosed},
		StateDraining: {StateClosed},
	}

	for _, valid := range validTransitions[from] {
		if valid == to {
			return true
		}
	}
	return false
}

func (sm *StateMachine) OnStateChange(state ConnectionState, fn func(ConnectionState) error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onChange[state] = fn
}

func (sm *StateMachine) IsReady() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateReady
}

func (sm *StateMachine) CanClose() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateReady || sm.state == StateClosing
}

type ServerStateMachine struct {
	sm         *StateMachine
	mu         sync.RWMutex
	uptime    time.Time
	drainStart time.Time
	draining atomic.Bool
	shutdown atomic.Bool
	listeners []chan ConnectionState
}

func NewServerStateMachine() *ServerStateMachine {
	ssm := &ServerStateMachine{
		sm:      NewStateMachine(),
		uptime:  time.Now(),
	}

	ssm.sm.OnStateChange(StateReady, func(s ConnectionState) error {
		return nil
	})

	ssm.sm.OnStateChange(StateClosing, func(s ConnectionState) error {
		ssm.draining.Store(true)
		ssm.drainStart = time.Now()
		return nil
	})

	ssm.sm.OnStateChange(StateClosed, func(s ConnectionState) error {
		ssm.draining.Store(false)
		return nil
	})

	return ssm
}

func (ssm *ServerStateMachine) Start() error {
	return ssm.sm.SetState(StateConnecting)
}

func (ssm *ServerStateMachine) Ready() error {
	return ssm.sm.SetState(StateReady)
}

func (ssm *ServerStateMachine) GracefulShutdown(timeout time.Duration) error {
	if err := ssm.sm.SetState(StateClosing); err != nil {
		return err
	}

	ssm.draining.Store(true)

	<-time.After(timeout)

	return ssm.sm.SetState(StateClosed)
}

func (ssm *ServerStateMachine) ForceClose() error {
	return ssm.sm.SetState(StateClosed)
}

func (ssm *ServerStateMachine) IsDraining() bool {
	return ssm.draining.Load()
}

func (ssm *ServerStateMachine) Uptime() time.Duration {
	return time.Since(ssm.uptime)
}

func (ssm *ServerStateMachine) Subscribe(ch chan ConnectionState) {
	ssm.mu.Lock()
	defer ssm.mu.Unlock()
	ssm.listeners = append(ssm.listeners, ch)
}

type HealthServer struct {
	mu          sync.RWMutex
	ready      atomic.Bool
	checks    map[string]HealthCheck
	startTime time.Time
}

type HealthCheck struct {
	Name    string
	Fn     func() error
	Timeout time.Duration
}

func NewHealthServer() *HealthServer {
	return &HealthServer{
		ready:   atomic.Bool{},
		checks: make(map[string]HealthCheck),
	}
}

func (h *HealthServer) Register(name string, fn func() error, timeout time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = HealthCheck{
		Name: name,
		Fn:  fn,
		Timeout: timeout,
	}
}

func (h *HealthServer) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","uptime":"%v"}`, time.Since(h.startTime))
}

func (h *HealthServer) Readiness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if !h.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready"}`)
		return
	}

	for _, check := range h.checks {
		err := check.Fn()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","check":"%s","error":"%v"}`, check.Name, err)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ready"}`)
}

func (h *HealthServer) SetReady(ready bool) {
	h.ready.Store(ready)
}

func (h *HealthServer) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.Liveness)
	mux.HandleFunc("/ready", h.Readiness)

	return mux
}

type CircuitBreaker struct {
	failures    atomic.Uint64
	successes  atomic.Uint64
	mu        sync.RWMutex
	state     CircuitState
	threshold uint64
	timeout   time.Duration
	lastFail  time.Time
	onOpen    func()
	onClose   func()
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (cb *CircuitBreaker) New(threshold uint64, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFail) > cb.timeout {
			cb.mu.Lock()
			cb.state = CircuitHalfOpen
			cb.mu.Unlock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) Success() {
	cb.successes.Add(1)
	cb.failures.Store(0)

	cb.mu.Lock()
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		if cb.onClose != nil {
			cb.onClose()
		}
	}
	cb.mu.Unlock()
}

func (cb *CircuitBreaker) Failure() {
	cb.failures.Add(1)
	cb.lastFail = time.Now()

	if cb.failures.Load() >= cb.threshold {
		cb.mu.Lock()
		cb.state = CircuitOpen
		if cb.onOpen != nil {
			cb.onOpen()
		}
		cb.mu.Unlock()
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) OnOpen(fn func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onOpen = fn
}

func (cb *CircuitBreaker) OnClose(fn func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onClose = fn
}