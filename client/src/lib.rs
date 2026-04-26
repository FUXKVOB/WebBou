// WebBou Protocol Library
// Собственный протокол без зависимости от WebSocket/WebTransport

pub mod webbou;

pub use webbou::{CryptoEngine, Frame, FrameFlags, FrameType, WebBouClient};
