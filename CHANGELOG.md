# Changelog

Все важные изменения в проекте WebBou документируются здесь.

## [v0.3.0] - 2026-04-19

### Добавлено

#### Протокол
- 0-RTT Handshake - быстрое переподключение с PSK
- Multi-Path QUIC - несколько сетевых путей
- Better Flow Control - продвинутый контроль потока
- Frame Types: HELLO, HELLO_ACK, HELLO_DONE, MULTI_PATH, PATH_CLOSE, FLOW_CONTROL, MAX_DATA, BLOCKED, ACK2
- Добавлен флаг VERSION 0x03

#### Безопасность
- Post-quantum криптография (Kyber-768 эмуляция + гибридный key exchange)
- TLS 1.3 support
- Certificate Pinning - защита от MITM
- Rate Limiting per-user
- DDoS Protection - лимиты и блокировка IP
- IP Reputation - score-based система

#### Производительность
- Batched I/O - batch отправка кадров
- Spinlock для hot path
- Memory Pool - преаллокация буферов
- Object Pooling - переиспользование объектов
- Back-pressure signals

#### Observability
- Structured JSON logging (DEBUG, INFO, WARN, ERROR, FATAL)
- Prometheus metrics endpoint (/metrics)
- Health endpoints (/health, /ready)

#### Управление
- State Machine - CONNECTED → CONNECTED → AUTH → READY → CLOSING → DRAINING → CLOSED
- Graceful Shutdown с drain timeout
- YAML конфиг с hot reload
- Circuit Breaker

#### Load Distribution
- Load Balancer (RoundRobin, LeastConnections)
- Backend Discovery
- Proxy Protocol support

#### Клиент (Rust)
- Config, ConnectionStateMachine, CircuitBreaker
- RetryPolicy, HealthChecker, MetricsCollector
- BatchedWriter, BackPressureController
- Interop тесты

### Изменено
- Обновлён протокол до версии 0x03
- Go 1.26.2
- Улучшен error handling

## [v0.2.0] - 2026-04-18

### Добавлено

#### Протокол
- 0-RTT Handshake - быстрое переподключение
- Multi-Path QUIC - несколько сетевых путей
- Better Flow Control - контроль потока
- Post-quantum криптография (Kyber + Dilithium)
- TLS 1.3 support
- Certificate Pinning

#### Безопасность
- Rate Limiting per-user
- IP Reputation система
- DDoS Protection

#### Производительность
- Batched I/O
- Spinlock
- Memory Pool
- Object Pooling
- Back-pressure

#### Observability
- Structured logging
- Prometheus metrics
- Health checks

#### Управление
- State machine
- Graceful shutdown
- YAML config + hot reload

#### Load Distribution
- Load Balancer
- Backend discovery

#### Клиент
- Расширенный config
- State machine
- Circuit breaker

### Изменено
- Улучшена производительность
- Добавлено больше метрик

## [v0.1.0] - 2026-04-15

### Добавлено
- Первый релиз WebBou протокола
- Go сервер с QUIC и TCP поддержкой
- Rust клиент с async/await
- ChaCha20-Poly1305 шифрование
- LZ4/Zstd компрессия
- GitHub Actions CI/CD
- Автоматические релизы
- Makefile
- Базовая документация

### Изменено
- Улучшена производительность с buffer pooling
- Добавлен автоматический reconnect
- Улучшена безопасность с rate limiting

[v0.3.0]: https://github.com/FUXKVOB/WebBou/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/FUXKVOB/WebBou/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/FUXKVOB/WebBou/releases/tag/v0.1.0