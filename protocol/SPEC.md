# WebBou Protocol Specification v1.1

## Обзор

WebBou - собственный бинарный протокол для высокоскоростной двунаправленной коммуникации, работающий поверх QUIC и TCP.

## Архитектура

```
┌─────────────────────────────────────────┐
│         WebBou Protocol Layer           │
├─────────────────────────────────────────┤
│  Frame Parser │ Multiplexer │ Crypto   │
├──────────────┬──────────────────────────┤
│   QUIC       │         TCP              │
│  (Primary)   │      (Fallback)        │
└──────────────┴──────────────────────────┘
```

## Frame Format

### Header (16 bytes)
```
  0                   1                   2                   3
  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |    Magic (0xB0)   |  Version  |    Type   |     Flags         |
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |                        Stream ID (32-bit)                     |
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |                      Payload Length (32-bit)                  |
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |                        Checksum (32-bit)                      |
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### Frame Types
- `0x01` - DATA: Обычные данные
- `0x02` - PING: Keepalive
- `0x03` - PONG: Ответ на PING
- `0x04` - STREAM_OPEN: Открыть новый stream
- `0x05` - STREAM_CLOSE: Закрыть stream
- `0x06` - ACK: Подтверждение
- `0x07` - RESET: Сброс соединения
- `0x08` - SETTINGS: Настройки
- `0x10` - HELLO: 0-RTT начало handshake
- `0x11` - HELLO_ACK: 0-RTT подтверждение
- `0x12` - HELLO_DONE: 0-RTT завершение
- `0x20` - MULTI_PATH: Multi-path управление
- `0x21` - PATH_CLOSE: Закрыть path
- `0x30` - FLOW_CONTROL: Flow control обновление
- `0x31` - MAX_DATA: Max data окно
- `0x32` - BLOCKED: Stream заблокирован
- `0x33` - ACK2: Расширенный ACK

### Flags
- `0x01` - COMPRESSED: Данные сжаты
- `0x02` - ENCRYPTED: Данные зашифрованы
- `0x04` - RELIABLE: Требуется ACK
- `0x08` - PRIORITY_HIGH: Высокий приоритет
- `0x10` - FRAGMENTED: Фрагментированное сообщение
- `0x20` - FINAL: Последний фрагмент
- `0x40` - ZERO_RTT: 0-RTT данные
- `0x80` - MULTI_PATH: Multi-path фрейм
- `0x100` - RESUMED: Сессия возобновлена
- `0x200` - ACK_EAGER: Немедленный ACK
- `0x400` - PTO: Probe Timeout

## 0-RTT Handshake

### Быстрое переподключение
```
Client                                Server
  |                                      |
  |--- HELLO (0-RTT, PSK) ------------->|
  |--- EARLY DATA (encrypted) ---------->|
  |                                      |
  |<-- HELLO_ACK (session_id, key) -----|
  |--- HELLO_DONE ------------------->|
  |                                      |
  |<======== DATA EXCHANGE ============>|
```

PSK хранится 24 часа, максимум 1000 сессий

## Multi-Path QUIC

### Множественные сетевые пути
- До 255 активных paths
- Автоматический выбор лучшего path по latency
- Fallback при отказе path

## Flow Control

### Продвинутый контроль потока
- Глобальный max_data (по умолчанию 16MB)
- Per-stream max_data (по умолчанию 16MB)
- BLOCKED сигнализация
- MAX_DATA обновления

## Handshake

```
Client                                Server
  |                                      |
  |--- HELLO (version, capabilities) -->|
  |                                      |
  |<-- HELLO_ACK (session_id, key) -----|
  |                                      |
  |--- AUTH (credentials) ------------->|
  |                                      |
  |<-- AUTH_OK (token) -----------------|
  |                                      |
  |<======== DATA EXCHANGE ============>|
```

## Security

### Post-quantum криптография
- TLS 1.3 с гибридным key exchange
- Kyber-768 (post-quantum) + Curve25519 (classical)
- ChaCha20-Poly1305AEAD
- Rotating session keys каждые 1000 frames

### Certificate Pinning
- SHA256 хеш сертификата
- Защита от MITM атак
- Проверка при каждом handshake

## Performance Features

- Zero-copy где возможно
- Lock-free queues для multiplexing
- Adaptive compression (LZ4/Zstd)
- Connection pooling
- Automatic MTU discovery
- 0-RTT для быстрого переподключения
- Batched I/O - batch отправка кадров
- Spinlock для hot path
- Memory pool - преаллокация буферов
- Object pool - переиспользование объектов

## HTTP/3 (QUIC) Ready

Сервер совместим с quic-go v0.59.0:
- QUIC listener на отдельном порту
- TCP fallback listener
- Datagram поддержка
- Stream multiplexing

## Wire Format Spec

Текущая версия: 1.1
- Magic: 0xB0
- Version: 0x01
- Header: 16 bytes fixed
- CRC32 checksums
- Big-endian encoding

## Interop Testing

Встроенные тесты:
- `test_spin_lock` - тест spinlock
- `test_memory_pool` - тест пула памяти
- `test_back_pressure` - тест back pressure
- `test_client_connection` - тест подключения
- `test_protocol_frame` - тест фреймов
- `test_encryption` - тест шифрования
- `test_reconnect_strategy` - тест переподключения

## Безопасность

### Rate Limiting
- Token bucket алгоритм
- Per-IP лимиты
- Connection лимиты

### DDoS Protection
- Лимит запросов в окно (100/сек по умолчанию)
- Блокировка IP на 5-10 минут
- Автоматическая очистка

### IP Reputation
- Score-based система
- Успешные запросы: +1
- Ошибочные запросы: -2
- Порог блокировки: -10