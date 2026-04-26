#[cfg(test)]
mod tests {
    use crate::webbou::batch::{BackPressureController, MemoryPool, SpinLock};
    use crate::webbou::compression::{compress, decompress};
    use crate::webbou::crypto::CryptoEngine;
    use crate::webbou::protocol::{Frame, FrameType};
    use crate::webbou::reconnect::ConnectionHealth;
    #[test]
    fn test_spin_lock() {
        let lock = SpinLock::new();

        lock.lock();
        assert!(lock.is_locked());

        lock.unlock();
        assert!(!lock.is_locked());
    }

    #[test]
    fn test_memory_pool() {
        let pool: MemoryPool<Vec<u8>> = MemoryPool::with_capacity(10);

        let buf1 = pool.get();
        assert!(buf1.is_empty());

        pool.put(vec![1, 2, 3]);

        let buf2 = pool.get();
        assert_eq!(buf2, vec![1, 2, 3]);
    }

    #[test]
    fn test_back_pressure() {
        let bp = BackPressureController::new(100, 50);

        assert!(bp.try_acquire(50));
        assert!(!bp.is_paused());

        assert!(!bp.try_acquire(60));
        assert!(bp.is_paused());

        bp.release(100);
        assert!(!bp.is_paused());
    }

    #[test]
    fn test_protocol_frame() {
        let frame = Frame::new(FrameType::Data, 1, b"test data".to_vec());

        assert_eq!(frame.frame_type, FrameType::Data.to_u8());
        assert_eq!(frame.stream_id, 1);

        let data = frame.marshal();
        assert!(data.len() > 0);
    }

    #[test]
    fn test_compression() {
        let original: Vec<u8> = (0..1000).map(|i| (i % 256) as u8).collect();

        if let Ok(compressed) = compress(&original) {
            assert!(compressed.len() < original.len());

            if let Ok(decompressed) = decompress(&compressed) {
                assert_eq!(original, decompressed);
            }
        }
    }

    #[test]
    fn test_encryption() {
        let crypto = CryptoEngine::new();

        let plaintext = b"Hello, World!";
        let encrypted = crypto.encrypt(plaintext).unwrap();

        assert_ne!(plaintext.as_slice(), encrypted.as_slice());

        let decrypted = crypto.decrypt(&encrypted).unwrap();
        assert_eq!(plaintext.as_slice(), decrypted.as_slice());
    }

    #[tokio::test]
    async fn test_reconnect_strategy() {
        let mut strategy = crate::webbou::ReconnectStrategy::new(3);

        assert!(strategy.should_retry());

        strategy.wait().await;
        assert_eq!(strategy.get_current_retry(), 1);

        strategy.wait().await;
        assert_eq!(strategy.get_current_retry(), 2);

        strategy.wait().await;
        assert!(!strategy.should_retry());
    }

    #[test]
    fn test_health_check() {
        let health = ConnectionHealth::new(
            std::time::Duration::from_secs(1),
            std::time::Duration::from_secs(5),
        );

        assert!(!health.should_ping());
        assert!(health.is_healthy());
    }
}
