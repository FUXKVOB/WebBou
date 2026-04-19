use std::collections::VecDeque;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::time::Duration;

pub struct SpinLock {
    locked: AtomicBool,
}

impl SpinLock {
    pub fn new() -> Self {
        Self {
            locked: AtomicBool::new(false),
        }
    }

    pub fn lock(&self) {
        while self
            .locked
            .compare_exchange(false, true, Ordering::Acquire, Ordering::Relaxed)
            .is_err()
        {
            std::hint::spin_loop();
        }
    }

    pub fn unlock(&self) {
        self.locked.store(false, Ordering::Release);
    }
}

impl Default for SpinLock {
    fn default() -> Self {
        Self::new()
    }
}

pub struct MemoryPool<T> {
    pool: Mutex<VecDeque<T>>,
    factory: Box<dyn Fn() -> T + Send + Sync>,
    max_size: usize,
    #[allow(dead_code)]
    current_size: AtomicU64,
}

impl<T: Default> MemoryPool<T> {
    pub fn with_capacity(max_size: usize) -> Self {
        Self {
            pool: Mutex::new(VecDeque::with_capacity(max_size)),
            factory: Box::new(|| T::default()),
            max_size,
            current_size: AtomicU64::new(0),
        }
    }

    pub fn get(&self) -> T {
        let mut pool = self.pool.lock().unwrap();
        pool.pop_front().unwrap_or_else(|| (self.factory)())
    }

    pub fn put(&self, item: T) {
        let mut pool = self.pool.lock().unwrap();
        if pool.len() < self.max_size {
            pool.push_back(item);
        }
    }
}

pub struct BackPressureController {
    enabled: AtomicBool,
    high_water_mark: u64,
    low_water_mark: u64,
    current_usage: AtomicU64,
    paused: AtomicBool,
}

impl BackPressureController {
    pub fn new(high_water_mark: u64, low_water_mark: u64) -> Self {
        Self {
            enabled: AtomicBool::new(true),
            high_water_mark,
            low_water_mark,
            current_usage: AtomicU64::new(0),
            paused: AtomicBool::new(false),
        }
    }

    pub fn try_acquire(&self, amount: u64) -> bool {
        if !self.enabled.load(Ordering::SeqCst) {
            return true;
        }

        let current = self.current_usage.load(Ordering::SeqCst);
        if current + amount > self.high_water_mark {
            self.paused.store(true, Ordering::SeqCst);
            return false;
        }

        self.current_usage.fetch_add(amount, Ordering::SeqCst);
        true
    }

    pub fn release(&self, amount: u64) {
        self.current_usage.fetch_sub(amount, Ordering::SeqCst);

        let current = self.current_usage.load(Ordering::SeqCst);
        if current < self.low_water_mark && self.paused.load(Ordering::SeqCst) {
            self.paused.store(false, Ordering::SeqCst);
        }
    }

    pub fn is_paused(&self) -> bool {
        self.paused.load(Ordering::SeqCst)
    }

    pub fn usage(&self) -> u64 {
        self.current_usage.load(Ordering::SeqCst)
    }

    pub fn enable(&self) {
        self.enabled.store(true, Ordering::SeqCst);
    }

    pub fn disable(&self) {
        self.enabled.store(false, Ordering::SeqCst);
    }
}

pub struct BatchedWriter {
    queue: Mutex<VecDeque<Vec<u8>>>,
    condvar: Condvar,
    shutdown: AtomicBool,
    pending: AtomicU64,
}

impl BatchedWriter {
    pub fn new() -> Self {
        Self {
            queue: Mutex::new(VecDeque::new()),
            condvar: Condvar::new(),
            shutdown: AtomicBool::new(false),
            pending: AtomicU64::new(0),
        }
    }

    pub fn enqueue(&self, data: Vec<u8>) {
        let mut queue = self.queue.lock().unwrap();
        queue.push_back(data);
        self.pending.fetch_add(1, Ordering::SeqCst);
        self.condvar.notify_one();
    }

    pub fn flush(&self) {
        let mut queue = self.queue.lock().unwrap();
        while let Some(data) = queue.pop_front() {
            self.pending.fetch_sub(1, Ordering::SeqCst);
            // In real implementation, write to socket
            let _ = data;
        }
    }

    pub fn shutdown(&self) {
        self.shutdown.store(true, Ordering::SeqCst);
        self.condvar.notify_all();
    }

    pub fn pending_count(&self) -> u64 {
        self.pending.load(Ordering::SeqCst)
    }
}

impl Default for BatchedWriter {
    fn default() -> Self {
        Self::new()
    }
}