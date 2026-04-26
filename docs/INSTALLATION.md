# WebBou Installation

This document describes the current production path in the repository.

## Current Runtime Contract

- transport: `TCP + TLS`
- server port: `8443`
- wire version: `v1`
- handshake: `HELLO -> HELLO_ACK`

## Prerequisites

### Windows

- Go `1.26+`
- Rust `1.77+`
- PowerShell
- Git

### Linux and macOS

- Go `1.26+`
- Rust `1.77+`
- OpenSSL
- Git

## Clone the Repository

```bash
git clone https://github.com/FUXKVOB/WebBou.git
cd WebBou
```

## Build

```bash
make all
```

## Generate Development Certificates

### Windows

```powershell
make dev-cert
```

This creates `cert.pem` and `key.pem` in the repository root.

### Linux and macOS

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
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

## Troubleshooting

### Missing certificate files

Generate `cert.pem` and `key.pem` before starting the server.

### Port already in use

Change `TCPAddr` in [main_webbou.go](/D:/WebBou/server/main_webbou.go).

### TLS verification in development

The Rust client currently accepts the local self-signed certificate for the development flow.
