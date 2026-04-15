# Установка WebBou

## Требования

### Для сервера (Go)
- Go 1.26 или выше
- Git

### Для клиента (Rust)
- Rust 1.77 или выше
- Cargo (устанавливается с Rust)
- Git

---

## Установка зависимостей

### Windows

#### 1. Установить Go
```powershell
# Скачать с официального сайта:
# https://go.dev/dl/

# Или через winget:
winget install GoLang.Go

# Проверить установку:
go version
```

#### 2. Установить Rust
```powershell
# Скачать rustup-init.exe с:
# https://rustup.rs/

# Или через winget:
winget install Rustlang.Rustup

# Проверить установку:
rustc --version
cargo --version
```

#### 3. Установить Git
```powershell
# Скачать с:
# https://git-scm.com/download/win

# Или через winget:
winget install Git.Git
```

### Linux (Ubuntu/Debian)

```bash
# Установить Go
wget https://go.dev/dl/go1.26.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Установить Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# Установить Git
sudo apt update
sudo apt install git
```

### macOS

```bash
# Установить Homebrew (если нет):
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Установить Go
brew install go

# Установить Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Установить Git
brew install git
```

---

## Скачивание WebBou

### Вариант 1: Готовые бинарники (рекомендуется)

Скачайте последнюю версию из [Releases](https://github.com/FUXKVOB/WebBou/releases):

- **Linux**: `webbou-linux-amd64.tar.gz`
- **Windows**: `webbou-windows-amd64.zip`
- **macOS**: `webbou-darwin-amd64.tar.gz`

```bash
# Linux/macOS
tar -xzf webbou-linux-amd64.tar.gz
cd webbou

# Windows
# Распаковать webbou-windows-amd64.zip
```

### Вариант 2: Сборка из исходников

```bash
# Клонировать репозиторий
git clone https://github.com/FUXKVOB/WebBou.git
cd WebBou
```

---

## Сборка

### Используя Makefile (Linux/macOS/Windows с WSL)

```bash
# Собрать всё (сервер + клиент)
make all

# Или по отдельности:
make server  # Только сервер
make client  # Только клиент

# Бинарники будут в папке bin/
# - bin/server
# - bin/client
```

### Ручная сборка

#### Сервер (Go)
```bash
cd server
go mod download
go build -o ../bin/server main_webbou.go
```

#### Клиент (Rust)
```bash
cd client
cargo build --release
cp target/release/webbou-client ../bin/client
```

---

## Генерация сертификатов (для QUIC/TLS)

### Windows

```powershell
# Создать самоподписанный сертификат
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"

# Или использовать PowerShell:
New-SelfSignedCertificate -DnsName "localhost" -CertStoreLocation "cert:\LocalMachine\My"
```

### Linux/macOS

```bash
# Установить OpenSSL (если нет)
# Ubuntu/Debian:
sudo apt install openssl

# macOS:
brew install openssl

# Создать сертификат
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

---

## Запуск

### Запустить сервер

```bash
# Используя Makefile
make run-server

# Или напрямую
./bin/server        # Linux/macOS
bin\server.exe      # Windows
```

Сервер запустится на:
- QUIC: `0.0.0.0:8443`
- TCP: `0.0.0.0:8444`

### Запустить клиент (в новом терминале)

```bash
# Используя Makefile
make run-client

# Или напрямую
./bin/client        # Linux/macOS
bin\client.exe      # Windows
```

---

## Использование в своём проекте

### Rust проект

Добавьте в `Cargo.toml`:

```toml
[dependencies]
webbou = { git = "https://github.com/FUXKVOB/WebBou.git", package = "webbou-client" }
tokio = { version = "1.52", features = ["full"] }
```

Пример кода:

```rust
use webbou::WebBouClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = WebBouClient::new("localhost:8443".to_string());
    client.connect().await?;
    
    // Отправить данные
    client.send(b"Hello!".to_vec(), true, false, false).await?;
    
    // Получить ответ
    let response = client.recv().await?;
    println!("Received: {:?}", String::from_utf8_lossy(&response));
    
    client.close().await?;
    Ok(())
}
```

### Go проект

```bash
# Добавить модуль
go get github.com/FUXKVOB/WebBou/server/webbou
```

Пример кода:

```go
package main

import (
    "log"
    "github.com/FUXKVOB/WebBou/server/webbou"
)

func main() {
    // Использовать клиентскую библиотеку
    // (TODO: добавить Go клиент)
}
```

---

## Проверка установки

```bash
# Проверить версии
go version      # должно быть >= 1.26
rustc --version # должно быть >= 1.77
cargo --version

# Проверить сборку
cd WebBou
make all

# Должны появиться файлы:
# bin/server (или bin/server.exe)
# bin/client (или bin/client.exe)
```

---

## Решение проблем

### "go: command not found"
- Убедитесь, что Go установлен и добавлен в PATH
- Перезапустите терминал после установки

### "cargo: command not found"
- Убедитесь, что Rust установлен
- Выполните: `source ~/.cargo/env` (Linux/macOS)
- Перезапустите терминал (Windows)

### "certificate verify failed"
- Используйте самоподписанный сертификат (см. раздел выше)
- Или отключите проверку сертификата в тестовом окружении

### Порты заняты
- Измените порты в `server/main_webbou.go`:
```go
QUICAddr: "0.0.0.0:9443",  // вместо 8443
TCPAddr:  "0.0.0.0:9444",  // вместо 8444
```

---

## Дополнительно

- [Быстрый старт](QUICKSTART.md)
- [Спецификация протокола](../protocol/SPEC.md)
- [Примеры использования](../examples/)
