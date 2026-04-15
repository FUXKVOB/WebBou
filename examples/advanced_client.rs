// Продвинутый пример с автопереподключением и сжатием

use webbou::WebBouClient;
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Создать клиент с автопереподключением
    let client = WebBouClient::new("localhost:8443".to_string())
        .with_auto_reconnect(true);
    
    println!("🚀 Подключение к WebBou серверу...");
    client.connect().await?;
    println!("✓ Подключено!\n");

    // Пример 1: Обычное сообщение
    println!("📤 Отправка обычного сообщения...");
    client.send(
        b"Hello, World!".to_vec(),
        true,   // reliable - гарантированная доставка
        false,  // compress - без сжатия
        false   // encrypt - без шифрования
    ).await?;
    
    let response = client.recv().await?;
    println!("📥 Ответ: {}\n", String::from_utf8_lossy(&response));

    // Пример 2: Большое сообщение со сжатием
    println!("📤 Отправка большого сообщения со сжатием...");
    let large_data = "Повторяющиеся данные! ".repeat(100).into_bytes();
    println!("   Размер до сжатия: {} байт", large_data.len());
    
    client.send(
        large_data,
        true,   // reliable
        true,   // compress - включить сжатие
        false   // encrypt
    ).await?;
    
    let response = client.recv().await?;
    println!("📥 Получено: {} байт\n", response.len());

    // Пример 3: Зашифрованное сообщение
    println!("🔒 Отправка зашифрованного сообщения...");
    client.send(
        b"Секретные данные".to_vec(),
        true,   // reliable
        false,  // compress
        true    // encrypt - включить шифрование
    ).await?;
    
    let response = client.recv().await?;
    println!("📥 Расшифровано: {}\n", String::from_utf8_lossy(&response));

    // Пример 4: Отправка с повторными попытками
    println!("🔄 Отправка с автоматическими повторами...");
    client.send_with_retry(
        b"Важное сообщение".to_vec(),
        true,
        false,
        false
    ).await?;
    println!("✓ Доставлено с гарантией\n");

    // Пример 5: Проверка задержки
    println!("⏱️  Проверка задержки...");
    let latency = client.ping().await?;
    println!("   Ping: {}ms\n", latency);

    // Пример 6: Получение с таймаутом
    println!("⏰ Получение с таймаутом...");
    match client.recv_with_timeout(Duration::from_secs(5)).await {
        Ok(data) => println!("📥 Получено: {} байт", data.len()),
        Err(_) => println!("⚠️  Таймаут - данных нет")
    }
    println!();

    // Показать статистику
    let stats = client.get_stats().await;
    println!("📊 Статистика:");
    println!("   Отправлено: {} байт ({} фреймов)", stats.bytes_sent, stats.frames_sent);
    println!("   Получено: {} байт ({} фреймов)", stats.bytes_recv, stats.frames_recv);
    println!("   Средняя задержка: {}ms", stats.avg_latency_ms);
    if stats.compression_ratio > 0.0 {
        println!("   Коэффициент сжатия: {:.2}x", 1.0 / stats.compression_ratio);
    }
    println!("   Переподключений: {}", stats.reconnect_count);

    // Проверить здоровье соединения
    if client.is_healthy().await {
        println!("\n✓ Соединение здорово");
    } else {
        println!("\n⚠️  Соединение нестабильно");
    }

    // Закрыть соединение
    client.close().await?;
    println!("\n👋 Соединение закрыто");

    Ok(())
}
