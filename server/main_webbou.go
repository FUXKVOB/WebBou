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
		QUICAddr:     "",
		TCPAddr:      "0.0.0.0:8443",
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

	log.Println("WebBou Server v1")
	log.Println("Minimal contract: TCP + TLS on :8443")

	if err := server.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}

	select {}
}
