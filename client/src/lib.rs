// WebBou Protocol Library
// Собственный протокол без зависимости от WebSocket/WebTransport

pub mod webbou;

pub use webbou::{WebBouClient, Frame, FrameType, FrameFlags, CryptoEngine};
