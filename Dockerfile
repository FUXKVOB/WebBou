# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY server/ ./server/
RUN cd server && go mod download && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server main_webbou.go security.go

FROM rust:1.85-alpine AS rust-builder
WORKDIR /app
COPY client/ ./client/
RUN cd client && cargo build --release --bin webbou-client

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=go-builder /out/server /app/server
COPY --from=rust-builder /app/client/target/release/webbou-client /app/webbou-client

EXPOSE 8443
ENTRYPOINT ["/app/server"]
