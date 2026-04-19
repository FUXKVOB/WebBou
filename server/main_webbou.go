package main

import (
	"crypto/tls"
	"log"
	"webbou/server/webbou"
)

func main() {
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatal("Failed to load certificates:", err)
	}

	config := &webbou.ServerConfig{
		QUICAddr:     "0.0.0.0:8443",
		TCPAddr:      "0.0.0.0:8444",
		MaxStreams:   1000,
		MaxFrameSize: 65536,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		},
	}

	server, err := webbou.NewServer(config)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	log.Println("╔═══════════════════════════════════════╗")
	log.Println("║       WebBou Server v1.0              ║")
	log.Println("║  Собственный протокол Go + Rust       ║")
	log.Println("╚═══════════════════════════════════════╝")
	log.Println()

	if err := server.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}

	// Block forever
	select {}
}
