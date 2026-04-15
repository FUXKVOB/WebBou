package webbou

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type Server struct {
	quicListener      net.Listener
	tcpListener       net.Listener
	sessions          sync.Map
	config            *Config
	crypto            *CryptoEngine
	rateLimiter       *RateLimiter
	connLimiter       *ConnectionRateLimiter
	sessionManager    *SessionManager
	bufferPool        *BufferPool
	framePool         *FramePool
}

type Config struct {
	QUICAddr         string
	TCPAddr          string
	MaxStreams       int
	MaxFrameSize     int
	CompressionLevel int
	TLSConfig        *tls.Config
}

type Session struct {
	ID        string
	conn      net.Conn
	streams   sync.Map
	sendQueue chan *Frame
	recvQueue chan *Frame
	ctx       context.Context
	cancel    context.CancelFunc
	stats     *SessionStats
}

type SessionStats struct {
	BytesSent     uint64
	BytesRecv     uint64
	FramesSent    uint64
	FramesRecv    uint64
	StreamsActive int
	CreatedAt     time.Time
}

func NewServer(config *Config) (*Server, error) {
	crypto, err := NewCryptoEngine()
	if err != nil {
		return nil, err
	}

	return &Server{
		config:         config,
		crypto:         crypto,
		rateLimiter:    NewRateLimiter(1000, 100), // 1000 tokens, 100/sec refill
		connLimiter:    NewConnectionRateLimiter(10), // 10 connections per IP
		sessionManager: NewSessionManager(30 * time.Minute),
		bufferPool:     NewBufferPool(8192),
		framePool:      NewFramePool(),
	}, nil
}

func (s *Server) Start() error {
	// Start QUIC listener
	go s.startQUIC()
	
	// Start TCP fallback listener
	go s.startTCP()
	
	log.Println("WebBou server started")
	log.Printf("  QUIC: %s", s.config.QUICAddr)
	log.Printf("  TCP:  %s", s.config.TCPAddr)
	
	return nil
}

func (s *Server) startQUIC() {
	quicConfig := &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 10 * time.Second,
		EnableDatagrams: true,
		// v0.58.0: MaxIncomingStreams moved to Transport
	}

	listener, err := quic.ListenAddr(s.config.QUICAddr, s.config.TLSConfig, quicConfig)
	if err != nil {
		log.Printf("QUIC listener failed: %v", err)
		return
	}

	log.Println("QUIC listener ready (quic-go v0.59.0)")

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("QUIC accept error: %v", err)
			continue
		}

		go s.handleQUICConnection(conn)
	}
}

func (s *Server) startTCP() {
	listener, err := tls.Listen("tcp", s.config.TCPAddr, s.config.TLSConfig)
	if err != nil {
		log.Printf("TCP listener failed: %v", err)
		return
	}

	log.Println("TCP listener ready")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("TCP accept error: %v", err)
			continue
		}

		go s.handleTCPConnection(conn)
	}
}

func (s *Server) handleQUICConnection(conn *quic.Conn) {
	session := s.createSession(conn.RemoteAddr().String())
	defer s.closeSession(session)

	log.Printf("QUIC connection from %s", conn.RemoteAddr())

	ctx := context.Background()

	// Handle streams (v0.59.0: AcceptStream returns quic.Stream interface)
	go func() {
		for {
			stream, err := conn.AcceptStream(ctx)
			if err != nil {
				return
			}
			go s.handleStream(session, stream)
		}
	}()

	// Handle datagrams
	for {
		data, err := conn.ReceiveDatagram(ctx)
		if err != nil {
			break
		}
		s.handleDatagram(session, data)
	}
}

func (s *Server) handleTCPConnection(conn net.Conn) {
	remoteIP := conn.RemoteAddr().String()

	// Check connection limit
	if !s.connLimiter.AllowConnection(remoteIP) {
		log.Printf("Connection limit reached for %s", remoteIP)
		conn.Close()
		return
	}
	defer s.connLimiter.ReleaseConnection(remoteIP)

	session := s.createSession(remoteIP)
	defer s.closeSession(session)

	log.Printf("TCP connection from %s", remoteIP)

	reader := NewFrameReader()
	buf := s.bufferPool.Get()
	defer s.bufferPool.Put(buf)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}

		reader.Feed(buf[:n])

		for {
			frame, err := reader.ReadFrame()
			if err != nil {
				log.Printf("Frame read error: %v", err)
				return
			}
			if frame == nil {
				break
			}

			// Rate limiting
			if !s.rateLimiter.AllowIP(remoteIP) {
				log.Printf("Rate limit exceeded for %s", remoteIP)
				continue
			}

			s.handleFrame(session, frame, conn)
		}
	}
}

func (s *Server) handleStream(session *Session, stream *quic.Stream) {
	defer stream.Close()

	reader := NewFrameReader()
	buf := make([]byte, 4096)

	for {
		n, err := stream.Read(buf)
		if err != nil {
			break
		}

		reader.Feed(buf[:n])

		for {
			frame, err := reader.ReadFrame()
			if err != nil {
				return
			}
			if frame == nil {
				break
			}

			s.handleFrame(session, frame, stream)
		}
	}
}

func (s *Server) handleDatagram(session *Session, data []byte) {
	frame, err := UnmarshalFrame(data)
	if err != nil {
		log.Printf("Invalid datagram: %v", err)
		return
	}

	s.handleFrame(session, frame, nil)
}

func (s *Server) handleFrame(session *Session, frame *Frame, writer interface{}) {
	session.stats.FramesRecv++
	session.stats.BytesRecv += uint64(len(frame.Payload))

	switch frame.Type {
	case FrameData:
		s.handleDataFrame(session, frame, writer)
	case FramePing:
		s.handlePingFrame(session, frame, writer)
	case FrameStreamOpen:
		s.handleStreamOpen(session, frame)
	case FrameStreamClose:
		s.handleStreamClose(session, frame)
	default:
		log.Printf("Unknown frame type: 0x%02x", frame.Type)
	}
}

func (s *Server) handleDataFrame(session *Session, frame *Frame, writer interface{}) {
	// Decrypt if needed
	if frame.HasFlag(FlagEncrypted) {
		decrypted, err := s.crypto.Decrypt(frame.Payload)
		if err != nil {
			log.Printf("Decryption failed: %v", err)
			return
		}
		frame.Payload = decrypted
	}

	// Decompress if needed
	if frame.HasFlag(FlagCompressed) {
		decompressed, err := Decompress(frame.Payload)
		if err != nil {
			log.Printf("Decompression failed: %v", err)
			return
		}
		frame.Payload = decompressed
	}

	// Echo back
	response := NewFrame(FrameData, frame.StreamID, frame.Payload)
	s.sendFrame(session, response, writer)
}

func (s *Server) handlePingFrame(session *Session, frame *Frame, writer interface{}) {
	pong := NewFrame(FramePong, frame.StreamID, frame.Payload)
	s.sendFrame(session, pong, writer)
}

func (s *Server) handleStreamOpen(session *Session, frame *Frame) {
	session.streams.Store(frame.StreamID, true)
	session.stats.StreamsActive++
}

func (s *Server) handleStreamClose(session *Session, frame *Frame) {
	session.streams.Delete(frame.StreamID)
	session.stats.StreamsActive--
}

func (s *Server) sendFrame(session *Session, frame *Frame, writer interface{}) {
	data := frame.Marshal()

	session.stats.FramesSent++
	session.stats.BytesSent += uint64(len(data))

	switch w := writer.(type) {
	case net.Conn:
		w.Write(data)
	case *quic.Stream:
		w.Write(data)
	}
}

func (s *Server) createSession(remoteAddr string) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	
	session := &Session{
		ID:        generateSessionID(),
		sendQueue: make(chan *Frame, 1000),
		recvQueue: make(chan *Frame, 1000),
		ctx:       ctx,
		cancel:    cancel,
		stats: &SessionStats{
			CreatedAt: time.Now(),
		},
	}

	s.sessions.Store(session.ID, session)
	return session
}

func (s *Server) closeSession(session *Session) {
	session.cancel()
	s.sessions.Delete(session.ID)
	close(session.sendQueue)
	close(session.recvQueue)
}

func generateSessionID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
