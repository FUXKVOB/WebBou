# Multi-stage build for Go server
FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY server/ ./server/
RUN cd server && go mod download && go build -o /server main.go security.go

# Multi-stage build for Rust client
FROM rust:1.77-alpine AS rust-builder
WORKDIR /app
COPY client/ ./client/
RUN cd client && cargo build --release

# Final image
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=go-builder /server /app/
COPY --from=rust-builder /app/client/target/release/client /app/
COPY cert.pem key.pem /app/

EXPOSE 8443

CMD ["/app/server"]
