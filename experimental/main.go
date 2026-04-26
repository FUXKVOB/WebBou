package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

type HybridServer struct {
	wtServer *webtransport.Server
	wsUpgrader websocket.Upgrader
	sessions sync.Map
	metrics *Metrics
}

type Session struct {
	ID string
	Protocol string
	CreatedAt time.Time
	BytesSent uint64
	BytesRecv uint64
}

type Metrics struct {
	mu sync.RWMutex
	ActiveSessions int
	TotalMessages uint64
	WebTransportCount int
	WebSocketCount int
}

func NewHybridServer(certFile, keyFile string) (*HybridServer, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	server := &HybridServer{
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		metrics: &Metrics{},
	}

	server.wtServer = &webtransport.Server{
		H3: &http3.Server{
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion: tls.VersionTLS13,
			},
			QuicConfig: &quic.Config{
				MaxIdleTimeout: 30 * time.Second,
				KeepAlivePeriod: 10 * time.Second,
			},
		},
	}

	return server, nil
}

func (s *HybridServer) HandleWebTransport(w http.ResponseWriter, r *http.Request) {
	session, err := s.wtServer.Upgrade(w, r)
	if err != nil {
		log.Printf("WebTransport upgrade failed: %v", err)
		return
	}
	defer session.Close()

	s.metrics.mu.Lock()
	s.metrics.ActiveSessions++
	s.metrics.WebTransportCount++
	s.metrics.mu.Unlock()

	log.Printf("WebTransport session established from %s", r.RemoteAddr)

	ctx := context.Background()
	
	// Handle bidirectional streams
	go s.handleStreams(ctx, session)
	
	// Handle datagrams
	go s.handleDatagrams(ctx, session)

	<-session.Context().Done()
	
	s.metrics.mu.Lock()
	s.metrics.ActiveSessions--
	s.metrics.mu.Unlock()
}

func (s *HybridServer) handleStreams(ctx context.Context, session *webtransport.Session) {
	for {
		stream, err := session.AcceptStream(ctx)
		if err != nil {
			return
		}
		go s.processStream(stream)
	}
}

func (s *HybridServer) processStream(stream webtransport.Stream) {
	defer stream.Close()
	
	buf := make([]byte, 4096)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			return
		}
		
		s.metrics.mu.Lock()
		s.metrics.TotalMessages++
		s.metrics.mu.Unlock()
		
		// Echo back
		if _, err := stream.Write(buf[:n]); err != nil {
			return
		}
	}
}

func (s *HybridServer) handleDatagrams(ctx context.Context, session *webtransport.Session) {
	for {
		msg, err := session.ReceiveDatagram(ctx)
		if err != nil {
			return
		}
		
		// Process unreliable datagram
		go s.processDatagram(session, msg)
	}
}

func (s *HybridServer) processDatagram(session *webtransport.Session, data []byte) {
	// Echo back unreliable
	session.SendDatagram(data)
}

func (s *HybridServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.metrics.mu.Lock()
	s.metrics.ActiveSessions++
	s.metrics.WebSocketCount++
	s.metrics.mu.Unlock()

	log.Printf("WebSocket connection established from %s", r.RemoteAddr)

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		
		s.metrics.mu.Lock()
		s.metrics.TotalMessages++
		s.metrics.mu.Unlock()
		
		// Echo back
		if err := conn.WriteMessage(msgType, msg); err != nil {
			break
		}
	}

	s.metrics.mu.Lock()
	s.metrics.ActiveSessions--
	s.metrics.mu.Unlock()
}

func (s *HybridServer) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"active_sessions": %d,
		"total_messages": %d,
		"webtransport_sessions": %d,
		"websocket_sessions": %d
	}`, s.metrics.ActiveSessions, s.metrics.TotalMessages, 
		s.metrics.WebTransportCount, s.metrics.WebSocketCount)
}

func main() {
	server, err := NewHybridServer("cert.pem", "key.pem")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wt", server.HandleWebTransport)
	mux.HandleFunc("/ws", server.HandleWebSocket)
	mux.HandleFunc("/metrics", server.HandleMetrics)

	log.Println("HybridTransport server starting on :8443")
	log.Fatal(http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", mux))
}
