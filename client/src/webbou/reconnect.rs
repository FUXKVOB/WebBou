use std::time::Duration;
use tokio::time::sleep;
use tracing::warn;

pub struct ReconnectStrategy {
    #[allow(dead_code)]
    max_retries: u32,
    #[allow(dead_code)]
    base_delay: Duration,
    #[allow(dead_code)]
    max_delay: Duration,
    current_retry: u32,
}

impl ReconnectStrategy {
    pub fn new(max_retries: u32) -> Self {
        Self {
            max_retries,
            base_delay: Duration::from_millis(100),
            max_delay: Duration::from_secs(30),
            current_retry: 0,
        }
    }

    pub fn reset(&mut self) {
        self.current_retry = 0;
    }

    #[allow(dead_code)]
    pub fn should_retry(&self) -> bool {
        self.current_retry < self.max_retries
    }

    #[allow(dead_code)]
    pub async fn wait(&mut self) {
        if self.current_retry >= self.max_retries {
            return;
        }

        // Exponential backoff: delay = base * 2^retry
        let delay_ms = self.base_delay.as_millis() * (1 << self.current_retry);
        let delay = Duration::from_millis(delay_ms.min(self.max_delay.as_millis()) as u64);

        warn!(
            "Reconnect attempt {} of {}, waiting {:?}",
            self.current_retry + 1,
            self.max_retries,
            delay
        );

        sleep(delay).await;
        self.current_retry += 1;
    }

    #[allow(dead_code)]
    pub fn get_current_retry(&self) -> u32 {
        self.current_retry
    }
}

pub struct ConnectionHealth {
    last_ping: std::time::Instant,
    last_pong: std::time::Instant,
    #[allow(dead_code)]
    ping_interval: Duration,
    #[allow(dead_code)]
    timeout: Duration,
}

impl ConnectionHealth {
    pub fn new(ping_interval: Duration, timeout: Duration) -> Self {
        let now = std::time::Instant::now();
        Self {
            last_ping: now,
            last_pong: now,
            ping_interval,
            timeout,
        }
    }

    #[allow(dead_code)]
    pub fn should_ping(&self) -> bool {
        self.last_ping.elapsed() >= self.ping_interval
    }

    pub fn mark_ping_sent(&mut self) {
        self.last_ping = std::time::Instant::now();
    }

    pub fn mark_pong_received(&mut self) {
        self.last_pong = std::time::Instant::now();
    }

    #[allow(dead_code)]
    pub fn is_healthy(&self) -> bool {
        self.last_pong.elapsed() < self.timeout
    }

    #[allow(dead_code)]
    pub fn get_latency(&self) -> Duration {
        if self.last_pong > self.last_ping {
            Duration::from_millis(0)
        } else {
            self.last_ping.duration_since(self.last_pong)
        }
    }
}
