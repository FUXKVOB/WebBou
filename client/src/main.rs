mod webbou;

use std::env;
use tracing::{info, warn, Level};
use tracing_subscriber;
use webbou::WebBouClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt().with_max_level(Level::INFO).init();

    let insecure_tls = env::var("WEBBOU_INSECURE_TLS")
        .map(|v| v == "1" || v.eq_ignore_ascii_case("true"))
        .unwrap_or(false);
    let mut client = WebBouClient::new("localhost:8443".to_string());
    if insecure_tls {
        warn!("WEBBOU_INSECURE_TLS=1 set: TLS certificate validation is DISABLED");
        client = client.with_insecure_tls(true);
    }

    info!("Connecting to WebBou server...");
    client.connect().await?;

    // Test reliable messages
    info!("Sending reliable messages...");
    for i in 0..10 {
        let data = format!("Reliable message {}", i).into_bytes();
        client.send(data, true, false, false).await?;

        let response = client.recv().await?;
        info!("Received: {} bytes", response.len());
    }

    // Test compressed messages
    info!("Sending compressed messages...");
    let large_data = "Hello, WebBou! ".repeat(100).into_bytes();
    client.send(large_data, true, true, false).await?;

    let response = client.recv().await?;
    info!("Received compressed: {} bytes", response.len());

    // Test encrypted messages
    info!("Sending encrypted messages...");
    let secret_data = b"Secret message".to_vec();
    client.send(secret_data, true, false, true).await?;

    let response = client.recv().await?;
    info!(
        "Received encrypted: {:?}",
        String::from_utf8_lossy(&response)
    );

    // Ping test
    info!("Testing latency...");
    let latency = client.ping().await?;
    info!("Ping: {}ms", latency);

    // Show statistics
    let stats = client.get_stats().await;
    info!("Statistics:");
    info!("  Frames sent: {}", stats.frames_sent);
    info!("  Frames received: {}", stats.frames_recv);
    info!("  Bytes sent: {}", stats.bytes_sent);
    info!("  Bytes received: {}", stats.bytes_recv);
    info!("  Compression ratio: {:.2}", stats.compression_ratio);

    client.close().await?;
    info!("Connection closed");

    Ok(())
}
