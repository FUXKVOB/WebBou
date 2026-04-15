# WebBou - Собственный высокоскоростной протокол

Полностью собственный бинарный протокол для real-time коммуникации, построенный с нуля на Go + Rust.

## Архитектура

```
┌─────────────────────────────────────────────┐
│         WebBou Protocol Layer               │
├─────────────────────────────────────────────┤
│  Frame Parser │ Multiplexer │ Crypto       │
├──────────────┬──────────────────────────────┤
│   QUIC       │         TCP                  │
│  (Primary)   │      (Fallback)              │
└──────────────┴──────────────────────────────┘
```

## Компоненты

- **Server (Go)**: `server/webbou/` - QUIC + TCP сервер
- **Client (Rust)**: `client/src/webbou/` - Высокопроизводительный клиент
- **Protocol**: `protocol/SPEC.md` - Спецификация протокола

## Возможности

- ✅ Собственный бинарный протокол (без WebSocket/WebTransport)
- ✅ QUIC (UDP) + TCP fallback
- ✅ ChaCha20-Poly1305 шифрование
- ✅ LZ4/Zstd компрессия
- ✅ Множественные streams
- ✅ Reliable + Unreliable режимы
- ✅ CRC32 checksums
- ✅ Zero-copy оптимизации

## Версии библиотек (2026)

### Go (1.26)
- quic-go v0.59.0
- klauspost/compress v1.18.5
- lz4 v4.1.26
- golang.org/x/crypto v0.50.0
- golang.org/x/net v0.53.0

### Rust (1.77+)
- tokio v1.52.0
- futures v0.3.32
- serde v1.0.228
- thiserror v2.0.18
- tracing v0.1.44
- tracing-subscriber v0.3.23
- crc32fast v1.5.0
- lz4_flex v0.13
- chacha20poly1305 v0.10.1
- x25519-dalek v2.0.1
- rand v0.10.1

## Быстрый старт

### Windows (PowerShell)

```powershell
# Сборка
.\build.ps1

# Тесты
.\test.ps1

# Версии
.\versions.ps1

# Запуск сервера
.\bin\server.exe

# Запуск клиента (новый терминал)
.\bin\client.exe
```

### Linux/macOS (Bash)

```bash
# Сборка
make all

# Тесты
make test

# Версии
make versions

# Запуск сервера
make run-server

# Запуск клиента
make run-client
```

## Ручная сборка

### Go сервер
```bash
cd server
go mod download
go build -o ../bin/server main_webbou.go
```

### Rust клиент
```bash
cd client
cargo build --release
```
