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

## Установка

### Требования
- **Go 1.26+** - [Скачать](https://go.dev/dl/)
- **Rust 1.77+** - [Скачать](https://rustup.rs/)
- **Git** - [Скачать](https://git-scm.com/)

### Быстрая установка

```bash
# 1. Клонировать репозиторий
git clone https://github.com/FUXKVOB/WebBou.git
cd WebBou

# 2. Собрать проект
make all

# 3. Создать сертификаты (для QUIC/TLS)
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"

# 4. Запустить сервер
make run-server

# 5. Запустить клиент (новый терминал)
make run-client
```

### Или скачать готовые бинарники

Перейдите в [Releases](https://github.com/FUXKVOB/WebBou/releases) и скачайте для вашей ОС:
- `webbou-linux-amd64.tar.gz` - Linux
- `webbou-windows-amd64.zip` - Windows
- `webbou-darwin-amd64.tar.gz` - macOS

📖 **Подробная инструкция**: [docs/INSTALLATION.md](docs/INSTALLATION.md)

## Использование в своём проекте

### Rust

```toml
[dependencies]
webbou = { git = "https://github.com/FUXKVOB/WebBou.git", package = "webbou-client" }
tokio = { version = "1.52", features = ["full"] }
```

```rust
use webbou::WebBouClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = WebBouClient::new("localhost:8443".to_string());
    client.connect().await?;
    
    client.send(b"Hello!".to_vec(), true, false, false).await?;
    let response = client.recv().await?;
    
    client.close().await?;
    Ok(())
}
```

### Go

```bash
go get github.com/FUXKVOB/WebBou/server/webbou
```
