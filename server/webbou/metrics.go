package webbou

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	mu               sync.RWMutex
	counters         map[string]*Counter
	gauges           map[string]*Gauge
	histograms       map[string]*Histogram
	prometheusServer *http.Server
	startTime       time.Time
}

type Counter struct {
	value uint64
}

type Gauge struct {
	value atomic.Value
}

type Histogram struct {
	mu    sync.Mutex
	count uint64
	sum   float64
	min   float64
	max   float64
}

var (
	globalMetrics = &Metrics{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
		startTime:  time.Now(),
	}
	metricsEnabled bool
)

func NewMetrics() *Metrics {
	return &Metrics{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
		startTime:  time.Now(),
	}
}

func GetMetrics() *Metrics {
	return globalMetrics
}

func EnableMetrics(enabled bool) {
	metricsEnabled = enabled
}

func (m *Metrics) Counter(name string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, exists := m.counters[name]; exists {
		return c
	}

	c := &Counter{}
	m.counters[name] = c
	return c
}

func (m *Metrics) Gauge(name string) *Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()

	if g, exists := m.gauges[name]; exists {
		return g
	}

	g := &Gauge{}
	g.value.Store(float64(0))
	m.gauges[name] = g
	return g
}

func (m *Metrics) Histogram(name string) *Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, exists := m.histograms[name]; exists {
		return h
	}

	h := &Histogram{
		min: float64(^uint64(0) >> 1),
	}
	m.histograms[name] = h
	return h
}

func (c *Counter) Inc() {
	atomic.AddUint64(&c.value, 1)
}

func (c *Counter) Add(n uint64) {
	atomic.AddUint64(&c.value, n)
}

func (c *Counter) Value() uint64 {
	return atomic.LoadUint64(&c.value)
}

func (g *Gauge) Set(v float64) {
	g.value.Store(v)
}

func (g *Gauge) Add(v float64) {
	old := g.value.Load().(float64)
	g.value.Store(old + v)
}

func (g *Gauge) Value() float64 {
	return g.value.Load().(float64)
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sum += v
	if v < h.min {
		h.min = v
	}
	if v > h.max {
		h.max = v
	}
}

func (h *Histogram) Stats() (count uint64, sum, min, max float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.count, h.sum, h.min, h.max
}

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		m.mu.RLock()
		defer m.mu.RUnlock()

		uptime := time.Since(m.startTime).Seconds()

		fmt.Fprintf(w, "# HELP webbou_uptime_seconds Server uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE webbou_uptime_seconds gauge\n")
		fmt.Fprintf(w, "webbou_uptime_seconds %.2f\n\n", uptime)

		for name, c := range m.counters {
			fmt.Fprintf(w, "# HELP webbou_%s_total Total %s\n", name, name)
			fmt.Fprintf(w, "# TYPE webbou_%s_total counter\n", name)
			fmt.Fprintf(w, "webbou_%s_total %d\n\n", name, c.Value())
		}

		for name, g := range m.gauges {
			fmt.Fprintf(w, "# HELP webbou_%s Current %s\n", name, name)
			fmt.Fprintf(w, "# TYPE webbou_%s gauge\n", name)
			fmt.Fprintf(w, "webbou_%s %.2f\n\n", name, g.Value())
		}

		for name, h := range m.histograms {
			count, sum, min, max := h.Stats()
			fmt.Fprintf(w, "# HELP webbou_%s_seconds Histogram %s\n", name, name)
			fmt.Fprintf(w, "# TYPE webbou_%s_seconds histogram\n", name)
			fmt.Fprintf(w, "webbou_%s_seconds_count %d\n", name, count)
			fmt.Fprintf(w, "webbou_%s_seconds_sum %.6f\n", name, sum)
			fmt.Fprintf(w, "webbou_%s_seconds_min %.6f\n", name, min)
			fmt.Fprintf(w, "webbou_%s_seconds_max %.6f\n\n", name, max)
		}
	}
}

func (m *Metrics) StartPrometheus(addr string) error {
	m.prometheusServer = &http.Server{
		Addr:    addr,
		Handler: m.Handler(),
	}

	go m.prometheusServer.ListenAndServe()
	return nil
}

type MetricsMiddleware struct {
	metrics *Metrics
}

func NewMetricsMiddleware() *MetricsMiddleware {
	return &MetricsMiddleware{
		metrics: GetMetrics(),
	}
}

func (m *MetricsMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		m.metrics.Counter("requests").Inc()

		next.ServeHTTP(w, r)

		duration := time.Since(start).Seconds()
		m.metrics.Histogram("request_duration").Observe(duration)

		w.Header().Add("X-WebBou-Metrics", "true")
	})
}

var (
	Connections = globalMetrics.Counter("connections")
	FramesSent  = globalMetrics.Counter("frames_sent")
	FramesRecv = globalMetrics.Counter("frames_recv")
	BytesSent = globalMetrics.Counter("bytes_sent")
	BytesRecv = globalMetrics.Counter("bytes_recv")
	Errors    = globalMetrics.Counter("errors")
	Latency   = globalMetrics.Histogram("latency")
	Bandwidth = globalMetrics.Gauge("bandwidth_mbps")
)