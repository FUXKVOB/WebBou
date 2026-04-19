pub mod protocol;
pub mod client;
pub mod crypto;
pub mod compression;
pub mod heartbeat;
pub mod reconnect;
pub mod batch;
pub mod interop;
pub mod config;

pub use protocol::{Frame, FrameType, FrameFlags};
pub use client::{WebBouClient, ClientStats};
pub use crypto::CryptoEngine;
pub use heartbeat::HeartbeatManager;
pub use reconnect::{ReconnectStrategy, ConnectionHealth};
pub use batch::{SpinLock, MemoryPool, BackPressureController, BatchedWriter};
pub use config::{Config, ConnectionStateMachine, CircuitBreaker, RetryPolicy, HealthChecker, MetricsCollector};
