use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    pub server: ServerConfig,
    pub client: ClientConfig,
    pub limits: LimitsConfig,
    pub keepalive: KeepaliveConfig,
    pub metrics: MetricsConfig,
    pub logging: LoggingConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClientConfig {
    pub connect_timeout: u64,
    pub read_timeout: u64,
    pub write_timeout: u64,
    pub retry_attempts: u32,
    pub tls_enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LimitsConfig {
    pub max_frame_size: usize,
    pub max_concurrent_streams: usize,
    pub receive_window: u64,
    pub send_window: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeepaliveConfig {
    pub interval: u64,
    pub timeout: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricsConfig {
    pub enabled: bool,
    pub port: u16,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoggingConfig {
    pub level: String,
    pub json: bool,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            server: ServerConfig {
                host: "localhost".to_string(),
                port: 8443,
            },
            client: ClientConfig {
                connect_timeout: 10,
                read_timeout: 30,
                write_timeout: 30,
                retry_attempts: 3,
                tls_enabled: true,
            },
            limits: LimitsConfig {
                max_frame_size: 65536,
                max_concurrent_streams: 100,
                receive_window: 16 * 1024 * 1024,
                send_window: 16 * 1024 * 1024,
            },
            keepalive: KeepaliveConfig {
                interval: 10,
                timeout: 30,
            },
            metrics: MetricsConfig {
                enabled: false,
                port: 9090,
            },
            logging: LoggingConfig {
                level: "info".to_string(),
                json: true,
            },
        }
    }
}

impl Config {
    pub fn from_file(path: &str) -> Result<Self, Box<dyn std::error::Error>> {
        let content = std::fs::read_to_string(path)?;
        let config: Config = serde_yaml::from_str(&content)?;
        Ok(config)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ClientState {
    Closed = 0,
    Connecting = 1,
    Connected = 2,
    Auth = 3,
    Ready = 4,
    Closing = 5,
}

pub struct ConnectionStateMachine {
    state: Arc<AtomicU64>,
    on_change: RwLock<HashMap<u64, Box<dyn Fn(u64) + Send + Sync>>>,
}

impl ConnectionStateMachine {
    pub fn new() -> Self {
        Self {
            state: Arc::new(AtomicU64::new(0)),
            on_change: RwLock::new(HashMap::new()),
        }
    }

    pub fn current_state(&self) -> u64 {
        self.state.load(Ordering::SeqCst)
    }

    pub fn set_state(&self, new_state: u64) -> Result<(), &'static str> {
        let old = self.state.load(Ordering::SeqCst);

        if !Self::valid_transition(old, new_state) {
            return Err("invalid transition");
        }

        self.state.store(new_state, Ordering::SeqCst);

        if let Ok(handlers) = self.on_change.read() {
            if let Some(handler) = handlers.get(&new_state) {
                handler(new_state);
            }
        }

        Ok(())
    }

    fn valid_transition(from: u64, to: u64) -> bool {
        matches!(
            (from, to),
            (0, 1) | (1, 2) | (2, 3) | (3, 4) | (4, 5) | (5, 0) | (1, 0)
        )
    }

    pub fn is_ready(&self) -> bool {
        self.state.load(Ordering::SeqCst) == 4
    }
}

pub struct CircuitBreaker {
    failures: AtomicU64,
    successes: Arc<AtomicU64>,
    threshold: u64,
    timeout: Duration,
    state: Arc<AtomicU64>,
    last_failure: RwLock<Option<Instant>>,
}

impl CircuitBreaker {
    pub fn new(threshold: u64, timeout: Duration) -> Self {
        Self {
            failures: AtomicU64::new(0),
            successes: Arc::new(AtomicU64::new(0)),
            threshold,
            timeout,
            state: Arc::new(AtomicU64::new(0)),
            last_failure: RwLock::new(None),
        }
    }

    pub fn allow(&self) -> bool {
        let state = self.state.load(Ordering::SeqCst);

        match state {
            0 => true,
            1 => {
                if let Ok(last) = self.last_failure.read() {
                    if let Some(t) = *last {
                        if t.elapsed() > self.timeout {
                            self.state.store(2, Ordering::SeqCst);
                            return true;
                        }
                    }
                }
                false
            }
            2 => true,
            _ => false,
        }
    }

    pub fn success(&self) {
        self.successes.fetch_add(1, Ordering::SeqCst);
        self.failures.store(0, Ordering::SeqCst);

        if self.state.load(Ordering::SeqCst) == 2 {
            self.state.store(0, Ordering::SeqCst);
        }
    }

    pub fn failure(&self) {
        self.failures.fetch_add(1, Ordering::SeqCst);

        if let Ok(mut last) = self.last_failure.write() {
            *last = Some(Instant::now());
        }

        if self.failures.load(Ordering::SeqCst) >= self.threshold {
            self.state.store(1, Ordering::SeqCst);
        }
    }

    pub fn is_open(&self) -> bool {
        self.state.load(Ordering::SeqCst) == 1
    }
}

pub struct RetryPolicy {
    max_attempts: u32,
    initial_delay: Duration,
    max_delay: Duration,
    multiplier: f64,
}

impl RetryPolicy {
    pub fn new(max_attempts: u32) -> Self {
        Self {
            max_attempts,
            initial_delay: Duration::from_millis(100),
            max_delay: Duration::from_secs(30),
            multiplier: 2.0,
        }
    }

    pub fn should_retry(&self, attempt: u32) -> bool {
        attempt < self.max_attempts
    }

    pub fn delay(&self, attempt: u32) -> Duration {
        let delay = self.initial_delay.as_millis() as f64 * self.multiplier.powi(attempt as i32);
        let delay = Duration::from_millis(delay.min(self.max_delay.as_millis() as f64) as u64);
        delay
    }
}

pub struct HealthChecker {
    checks: RwLock<HashMap<String, Box<dyn Fn() -> bool + Send + Sync>>>,
}

impl HealthChecker {
    pub fn new() -> Self {
        Self {
            checks: RwLock::new(HashMap::new()),
        }
    }

    pub fn register<F>(&self, name: String, check: F)
    where
        F: Fn() -> bool + Send + Sync + 'static,
    {
        if let Ok(mut checks) = self.checks.write() {
            checks.insert(name, Box::new(check));
        }
    }

    pub fn check(&self) -> bool {
        if let Ok(checks) = self.checks.read() {
            for (_, check) in checks.iter() {
                if !check() {
                    return false;
                }
            }
        }
        true
    }
}

pub struct MetricsCollector {
    connections: AtomicU64,
    frames_sent: AtomicU64,
    frames_recv: AtomicU64,
    bytes_sent: AtomicU64,
    bytes_recv: AtomicU64,
    errors: AtomicU64,
    latency_sum: AtomicU64,
    active_connections: AtomicU64,
}

impl MetricsCollector {
    pub fn new() -> Self {
        Self {
            connections: AtomicU64::new(0),
            frames_sent: AtomicU64::new(0),
            frames_recv: AtomicU64::new(0),
            bytes_sent: AtomicU64::new(0),
            bytes_recv: AtomicU64::new(0),
            errors: AtomicU64::new(0),
            latency_sum: AtomicU64::new(0),
            active_connections: AtomicU64::new(0),
        }
    }

    pub fn inc_connections(&self) {
        self.connections.fetch_add(1, Ordering::SeqCst);
        self.active_connections.fetch_add(1, Ordering::SeqCst);
    }

    pub fn dec_connections(&self) {
        self.active_connections.fetch_sub(1, Ordering::SeqCst);
    }

    pub fn inc_frames_sent(&self) {
        self.frames_sent.fetch_add(1, Ordering::SeqCst);
    }

    pub fn inc_frames_recv(&self) {
        self.frames_recv.fetch_add(1, Ordering::SeqCst);
    }

    pub fn add_bytes_sent(&self, n: u64) {
        self.bytes_sent.fetch_add(n, Ordering::SeqCst);
    }

    pub fn add_bytes_recv(&self, n: u64) {
        self.bytes_recv.fetch_add(n, Ordering::SeqCst);
    }

    pub fn inc_errors(&self) {
        self.errors.fetch_add(1, Ordering::SeqCst);
    }

    pub fn add_latency(&self, ms: u64) {
        self.latency_sum.fetch_add(ms, Ordering::SeqCst);
    }

    pub fn get(&self) -> MetricsSnapshot {
        MetricsSnapshot {
            connections: self.connections.load(Ordering::SeqCst),
            active_connections: self.active_connections.load(Ordering::SeqCst),
            frames_sent: self.frames_sent.load(Ordering::SeqCst),
            frames_recv: self.frames_recv.load(Ordering::SeqCst),
            bytes_sent: self.bytes_sent.load(Ordering::SeqCst),
            bytes_recv: self.bytes_recv.load(Ordering::SeqCst),
            errors: self.errors.load(Ordering::SeqCst),
        }
    }
}

#[derive(Debug, Clone)]
pub struct MetricsSnapshot {
    pub connections: u64,
    pub active_connections: u64,
    pub frames_sent: u64,
    pub frames_recv: u64,
    pub bytes_sent: u64,
    pub bytes_recv: u64,
    pub errors: u64,
}