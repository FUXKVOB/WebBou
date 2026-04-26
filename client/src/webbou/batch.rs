use std::collections::VecDeque;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Condvar, Mutex};

#[allow(dead_code)]
pub struct SpinLock {
    locked: AtomicBool,
}

impl SpinLock {
    #[allow(dead_code)]
    pub fn new() -> Self {
        Self {
            locked: AtomicBool::new(false),
        }
    }

    #[allow(dead_code)]
    pub fn lock(&self) {
        while self
            .locked
            .compare_exchange(false, true, Ordering::Acquire, Ordering::Relaxed)
            .is_err()
        {
            std::hint::spin_loop();
        }
    }

    #[allow(dead_code)]
    pub fn unlock(&self) {
        self.locked.store(false, Ordering::Release);
    }

    #[allow(dead_code)]
    pub fn is_locked(&self) -> bool {
        self.locked.load(Ordering::Acquire)
    }
}

impl Default for SpinLock {
    fn default() -> Self {
        Self::new()
    }
}

#[allow(dead_code)]
pub struct MemoryPool<T> {
    pool: Mutex<VecDeque<T>>,
    factory: Box<dyn Fn() -> T + Send + Sync>,
    max_size: usize,
    #[allow(dead_code)]
    current_size: AtomicU64,
}

impl<T: Default> MemoryPool<T> {
    #[allow(dead_code)]
    pub fn with_capacity(max_size: usize) -> Self {
        Self {
            pool: Mutex::new(VecDeque::with_capacity(max_size)),
            factory: Box::new(|| T::default()),
            max_size,
            current_size: AtomicU64::new(0),
        }
    }

    #[allow(dead_code)]
    pub fn get(&self) -> T {
        let mut pool = self.pool.lock().unwrap();
        pool.pop_front().unwrap_or_else(|| (self.factory)())
    }

    #[allow(dead_code)]
    pub fn put(&self, item: T) {
        let mut pool = self.pool.lock().unwrap();
        if pool.len() < self.max_size {
            pool.push_back(item);
        }
    }
}

#[allow(dead_code)]
pub struct BackPressureController {
    enabled: AtomicBool,
    high_water_mark: u64,
    low_water_mark: u64,
    current_usage: AtomicU64,
    paused: AtomicBool,
}

impl BackPressureController {
    #[allow(dead_code)]
    pub fn new(high_water_mark: u64, low_water_mark: u64) -> Self {
        Self {
            enabled: AtomicBool::new(true),
            high_water_mark,
            low_water_mark,
            current_usage: AtomicU64::new(0),
            paused: AtomicBool::new(false),
        }
    }

    #[allow(dead_code)]
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

    #[allow(dead_code)]
    pub fn release(&self, amount: u64) {
        let current = self.current_usage.load(Ordering::SeqCst);
        let new_usage = current.saturating_sub(amount);
        self.current_usage.store(new_usage, Ordering::SeqCst);

        if new_usage < self.low_water_mark && self.paused.load(Ordering::SeqCst) {
            self.paused.store(false, Ordering::SeqCst);
        }
    }

    #[allow(dead_code)]
    pub fn is_paused(&self) -> bool {
        self.paused.load(Ordering::SeqCst)
    }

    #[allow(dead_code)]
    pub fn usage(&self) -> u64 {
        self.current_usage.load(Ordering::SeqCst)
    }

    #[allow(dead_code)]
    pub fn enable(&self) {
        self.enabled.store(true, Ordering::SeqCst);
    }

    #[allow(dead_code)]
    pub fn disable(&self) {
        self.enabled.store(false, Ordering::SeqCst);
    }
}

#[allow(dead_code)]
pub struct BatchedWriter {
    queue: Mutex<VecDeque<Vec<u8>>>,
    condvar: Condvar,
    shutdown: AtomicBool,
    pending: AtomicU64,
}

impl BatchedWriter {
    #[allow(dead_code)]
    pub fn new() -> Self {
        Self {
            queue: Mutex::new(VecDeque::new()),
            condvar: Condvar::new(),
            shutdown: AtomicBool::new(false),
            pending: AtomicU64::new(0),
        }
    }

    #[allow(dead_code)]
    pub fn enqueue(&self, data: Vec<u8>) {
        let mut queue = self.queue.lock().unwrap();
        queue.push_back(data);
        self.pending.fetch_add(1, Ordering::SeqCst);
        self.condvar.notify_one();
    }

    #[allow(dead_code)]
    pub fn flush(&self) {
        let mut queue = self.queue.lock().unwrap();
        while let Some(data) = queue.pop_front() {
            self.pending.fetch_sub(1, Ordering::SeqCst);
            // In real implementation, write to socket
            let _ = data;
        }
    }

    #[allow(dead_code)]
    pub fn shutdown(&self) {
        self.shutdown.store(true, Ordering::SeqCst);
        self.condvar.notify_all();
    }

    #[allow(dead_code)]
    pub fn pending_count(&self) -> u64 {
        self.pending.load(Ordering::SeqCst)
    }
}

impl Default for BatchedWriter {
    fn default() -> Self {
        Self::new()
    }
}
