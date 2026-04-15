# WebBou Protocol

## Frame Format

```
Header: 16 bytes
┌────────┬─────────┬──────┬───────┬───────────┬────────┬──────────┐
│ Magic  │ Version │ Type │ Flags │ Stream ID │ Length │ Checksum │
│ 1 byte │ 1 byte  │1 byte│1 byte │  4 bytes  │4 bytes │ 4 bytes  │
└────────┴─────────┴──────┴───────┴───────────┴────────┴──────────┘
                            ↓
                        Payload
```

### Magic Byte: 0xB0
Идентификатор протокола WebBou

### Frame Types
- `0x01` DATA - Данные
- `0x02` PING - Keepalive
- `0x03` PONG - Ответ
- `0x04` STREAM_OPEN - Открыть stream
- `0x05` STREAM_CLOSE - Закрыть stream
- `0x06` ACK - Подтверждение
- `0x07` RESET - Сброс
- `0x08` SETTINGS - Настройки

### Flags
- `0x01` COMPRESSED - Сжато
- `0x02` ENCRYPTED - Зашифровано
- `0x04` RELIABLE - Требуется ACK
- `0x08` PRIORITY_HIGH - Высокий приоритет
- `0x10` FRAGMENTED - Фрагмент
- `0x20` FINAL - Последний фрагмент

## Transport

### QUIC (Primary)
- UDP-based
- Множественные streams
- 0-RTT reconnect

### TCP (Fallback)
- Надежная доставка
- Автоматический fallback

## Security

### Encryption
- Algorithm: ChaCha20-Poly1305
- Key size: 256 bits
- Nonce: 192 bits (XChaCha20)

### Key Exchange
- Curve25519
- Diffie-Hellman

### Checksums
- Algorithm: CRC32
- Covers: Header + Payload

## Performance

### Latency
- QUIC: 5-10ms (0-RTT)
- TCP: 15-30ms

### Throughput
- QUIC: 1-2 GB/s
- TCP: 500-800 MB/s

### Compression
- Text: 60-70% reduction
- JSON: 50-60% reduction
- Binary: 10-20% reduction
