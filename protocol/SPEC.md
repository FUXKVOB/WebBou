# WebBou Protocol Specification v1.0

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
│  (Primary)   │      (Fallback)          │
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

### Flags
- `0x01` - COMPRESSED: Данные сжаты
- `0x02` - ENCRYPTED: Данные зашифрованы
- `0x04` - RELIABLE: Требуется ACK
- `0x08` - PRIORITY_HIGH: Высокий приоритет
- `0x10` - FRAGMENTED: Фрагментированное сообщение
- `0x20` - FINAL: Последний фрагмент

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

- ChaCha20-Poly1305 шифрование
- BLAKE3 для checksums
- Curve25519 для key exchange
- Rotating session keys каждые 1000 frames

## Performance Features

- Zero-copy где возможно
- Lock-free queues для multiplexing
- Adaptive compression (LZ4/Zstd)
- Connection pooling
- Automatic MTU discovery
