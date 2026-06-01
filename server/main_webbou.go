package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"webbou/server/webbou"
)

func main() {
	certFile := flag.String("cert", "cert.pem", "TLS certificate file")
	keyFile := flag.String("key", "key.pem", "TLS private key file")
	tcpAddr := flag.String("tcp", "0.0.0.0:8443", "TCP listen address")
	quicAddr := flag.String("quic", "", "QUIC listen address (empty = disabled)")
	maxFrameSize := flag.Int("max-frame", 65536, "Maximum frame payload size in bytes")
	flag.Parse()

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("Failed to load certificates: %v", err)
	}

	config := &webbou.ServerConfig{
		QUICAddr:     *quicAddr,
		TCPAddr:      *tcpAddr,
		MaxStreams:   1000,
		MaxFrameSize: *maxFrameSize,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		},
	}

	server, err := webbou.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Println("WebBou Server v1")
	log.Println("Minimal contract: TCP + TLS on :8443")

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
		os.Exit(1)
	}
}
