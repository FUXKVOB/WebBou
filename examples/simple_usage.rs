// Простой пример использования WebBou клиента

use webbou::WebBouClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 1. Создать клиент
    let client = WebBouClient::new("localhost:8443".to_string());
    
    // 2. Подключиться к серверу
    println!("Подключение к серверу...");
    client.connect().await?;
    println!("✓ Подключено!");
    
    // 3. Отправить сообщение
    let message = b"Привет, WebBou!".to_vec();
    println!("Отправка: {:?}", String::from_utf8_lossy(&message));
    
    client.send(
        message,
        true,   // reliable - гарантированная доставка
        false,  // compress - без сжатия
        false   // encrypt - без шифрования
    ).await?;
    
    // 4. Получить ответ
    let response = client.recv().await?;
    println!("Получено: {:?}", String::from_utf8_lossy(&response));
    
    // 5. Закрыть соединение
    client.close().await?;
    println!("✓ Соединение закрыто");
    
    Ok(())
}
