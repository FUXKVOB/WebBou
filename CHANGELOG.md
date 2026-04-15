# Changelog

Все важные изменения в проекте WebBou документируются здесь.

## [Unreleased]

### Добавлено
- GitHub Actions CI/CD для автоматической сборки
- Автоматические релизы с бинарниками для Linux/Windows/macOS
- Makefile для удобной сборки
- Подробная документация по установке

### Изменено
- Улучшена производительность с buffer pooling
- Добавлен автоматический reconnect
- Улучшена безопасность с rate limiting

## [v0.1.0] - 2026-04-15

### Добавлено
- Первый релиз WebBou протокола
- Go сервер с QUIC и TCP поддержкой
- Rust клиент с async/await
- ChaCha20-Poly1305 шифрование
- LZ4/Zstd компрессия
- Базовая документация

[Unreleased]: https://github.com/FUXKVOB/WebBou/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/FUXKVOB/WebBou/releases/tag/v0.1.0
