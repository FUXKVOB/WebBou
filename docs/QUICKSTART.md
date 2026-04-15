# WebBou Quick Start

## Установка

```bash
# Требования: Go 1.26+, Rust 1.77+
make all
```

## Запуск

```bash
# Сервер
make run-server

# Клиент (другой терминал)
make run-client
```

## Примеры

### Базовое подключение

```rust
use webbou::WebBouClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = WebBouClient::new("localhost:8443".to_string());
    client.connect().await?;
    
    // Отправить (data, reliable, compress, encrypt)
    client.send(b"Hello!".to_vec(), true, false, false).await?;
    
    let response = client.recv().await?;
    println!("Received: {:?}", String::from_utf8_lossy(&response));
    
    Ok(())
}
```

### Сжатие и шифрование

```rust
// Сжатие (LZ4)
client.send(large_data, true, true, false).await?;

// Шифрование (ChaCha20-Poly1305)
client.send(secret_data, true, false, true).await?;
```

### Unreliable режим

```rust
// Быстрая отправка без гарантий
for i in 0..1000 {
    client.send(format!("Frame {}", i).into_bytes(), false, false, false).await?;
}
```

## Команды

```bash
make test           # Тесты
make benchmark      # Бенчмарки
make deps-update    # Обновить зависимости
make versions       # Показать версии
```
