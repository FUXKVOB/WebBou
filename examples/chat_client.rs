// Пример простого чат-клиента

use webbou::WebBouClient;
use std::io::{self, Write};
use tokio::time::{sleep, Duration};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("╔═══════════════════════════════════╗");
    println!("║     WebBou Chat Client v1.0       ║");
    println!("╚═══════════════════════════════════╝\n");

    // Подключение
    let client = WebBouClient::new("localhost:8443".to_string())
        .with_auto_reconnect(true);
    
    print!("Подключение к серверу... ");
    io::stdout().flush()?;
    
    client.connect().await?;
    println!("✓\n");

    // Запросить имя пользователя
    print!("Введите ваше имя: ");
    io::stdout().flush()?;
    
    let mut username = String::new();
    io::stdin().read_line(&mut username)?;
    let username = username.trim();

    println!("\n💬 Чат запущен! Введите сообщение или 'exit' для выхода\n");

    // Основной цикл чата
    loop {
        print!("{}: ", username);
        io::stdout().flush()?;

        let mut message = String::new();
        io::stdin().read_line(&mut message)?;
        let message = message.trim();

        if message.is_empty() {
            continue;
        }

        if message == "exit" || message == "quit" {
            break;
        }

        // Специальные команды
        if message.starts_with('/') {
            match message {
                "/ping" => {
                    match client.ping().await {
                        Ok(latency) => println!("   🏓 Pong! {}ms\n", latency),
                        Err(e) => println!("   ❌ Ошибка: {}\n", e),
                    }
                    continue;
                }
                "/stats" => {
                    let stats = client.get_stats().await;
                    println!("   📊 Статистика:");
                    println!("      Отправлено: {} байт", stats.bytes_sent);
                    println!("      Получено: {} байт", stats.bytes_recv);
                    println!("      Задержка: {}ms\n", stats.avg_latency_ms);
                    continue;
                }
                "/help" => {
                    println!("   📖 Команды:");
                    println!("      /ping  - проверить задержку");
                    println!("      /stats - показать статистику");
                    println!("      /help  - эта справка");
                    println!("      exit   - выйти из чата\n");
                    continue;
                }
                _ => {
                    println!("   ❓ Неизвестная команда. Используйте /help\n");
                    continue;
                }
            }
        }

        // Отправить сообщение
        let full_message = format!("{}: {}", username, message);
        
        match client.send_with_retry(
            full_message.into_bytes(),
            true,   // reliable
            true,   // compress для длинных сообщений
            false   // encrypt
        ).await {
            Ok(_) => {
                // Получить ответ от сервера
                match client.recv_with_timeout(Duration::from_secs(5)).await {
                    Ok(response) => {
                        if !response.is_empty() {
                            println!("   ✓ Сервер: {}\n", String::from_utf8_lossy(&response));
                        }
                    }
                    Err(_) => println!("   ⚠️  Нет ответа от сервера\n"),
                }
            }
            Err(e) => println!("   ❌ Ошибка отправки: {}\n", e),
        }
    }

    // Выход
    println!("\n👋 Выход из чата...");
    client.close().await?;
    println!("✓ До свидания!\n");

    Ok(())
}
