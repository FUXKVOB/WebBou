use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;
use tokio::time::interval;
use tracing::{debug, error, warn};

use super::protocol::{Frame, FrameType};

pub struct HeartbeatManager {
    interval: Duration,
    timeout: Duration,
    last_activity: Arc<RwLock<std::time::Instant>>,
    is_running: Arc<RwLock<bool>>,
}

impl HeartbeatManager {
    pub fn new(interval: Duration, timeout: Duration) -> Self {
        Self {
            interval,
            timeout,
            last_activity: Arc::new(RwLock::new(std::time::Instant::now())),
            is_running: Arc::new(RwLock::new(false)),
        }
    }

    pub async fn start<F, Fut>(&self, mut send_fn: F)
    where
        F: FnMut(Frame) -> Fut + Send + 'static,
        Fut: std::future::Future<Output = Result<(), Box<dyn std::error::Error + Send + Sync>>>
            + Send,
    {
        *self.is_running.write().await = true;

        let interval_duration = self.interval;
        let timeout_duration = self.timeout;
        let last_activity = Arc::clone(&self.last_activity);
        let is_running = Arc::clone(&self.is_running);

        tokio::spawn(async move {
            let mut ticker = interval(interval_duration);

            loop {
                ticker.tick().await;

                if !*is_running.read().await {
                    break;
                }

                let elapsed = last_activity.read().await.elapsed();

                if elapsed >= timeout_duration {
                    error!("Connection timeout - no activity for {:?}", elapsed);
                    break;
                }

                if elapsed >= interval_duration {
                    debug!("Sending heartbeat ping");

                    let ping_frame = Frame::new(FrameType::Ping, 0, vec![]);

                    if let Err(e) = send_fn(ping_frame).await {
                        warn!("Failed to send heartbeat: {}", e);
                        break;
                    }

                    *last_activity.write().await = std::time::Instant::now();
                }
            }

            *is_running.write().await = false;
        });
    }

    pub async fn mark_activity(&self) {
        *self.last_activity.write().await = std::time::Instant::now();
    }

    pub async fn stop(&self) {
        *self.is_running.write().await = false;
    }

    #[allow(dead_code)]
    pub async fn is_running(&self) -> bool {
        *self.is_running.read().await
    }

    #[allow(dead_code)]
    pub async fn get_last_activity(&self) -> Duration {
        self.last_activity.read().await.elapsed()
    }
}
