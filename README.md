# WebBou

WebBou is a minimal custom binary protocol stack with:

- Go server
- Rust client
- one production transport: `TCP + TLS`
- one production port: `8443`
- one wire format version: `v1`
- one handshake: `HELLO -> HELLO_ACK`

The repo also contains older experimental work, but the production path is the minimal contract above.

## Layout

- `server/webbou/`: Go server library
- `server/main_webbou.go`: production server binary
- `client/src/webbou/`: Rust client library
- `protocol/SPEC.md`: versioned wire spec
- `protocol/testdata/`: shared golden fixtures
- `experimental/`: research and legacy entrypoints

## Quick Start

1. Build the server and client.

```bash
make all
```

2. Generate a local certificate for development.

```powershell
make dev-cert
```

3. Start the server.

```bash
make run-server
```

4. Start the client in a second terminal.

```bash
make run-client
```

The Rust client currently accepts the local self-signed certificate so the development flow works without extra trust setup.
On Linux and macOS you can still generate the same files with `openssl`.

## Reality Check

Supported today:

- fixed 16-byte frame header
- frame version `0x01`
- `HELLO`, `HELLO_ACK`, `DATA`, `PING`, `PONG`, `STREAM_OPEN`, `STREAM_CLOSE`
- optional payload compression and payload encryption
- shared golden serialization tests between Go and Rust

Not part of the production contract yet:

- QUIC transport
- multi-path
- certificate pinning
- post-quantum key exchange
- 0-RTT resume

## Verification

The CI path is intentionally small and should stay green:

```bash
cd server && go build ./... && go test ./...
cd client && cargo build && cargo test
```

More detail is in [docs/QUICKSTART.md](docs/QUICKSTART.md).
