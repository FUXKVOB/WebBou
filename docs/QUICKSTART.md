# WebBou Quick Start

This document matches the production path in the repository today.

## Minimal Contract

- transport: `TCP + TLS`
- server port: `8443`
- wire version: `v1`
- handshake: `HELLO -> HELLO_ACK`

## Prerequisites

- Go `1.26+`
- Rust `1.77+`
- PowerShell on Windows, or OpenSSL on Linux/macOS

## Build

```bash
make all
```

## Generate Development Certificates

```powershell
make dev-cert
```

## Run

```bash
make run-server
make run-client
```

## Validate the Repo

```bash
cd server && go build ./... && go test ./...
cd client && cargo build && cargo test
```

## Notes

- The client uses TLS for the production path.
- The current development setup accepts the local self-signed certificate.
- Experimental binaries live outside the production flow.
- On Linux and macOS, `openssl` can generate the same `cert.pem` and `key.pem` files.
