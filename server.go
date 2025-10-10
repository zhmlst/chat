package chat

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/zhmlst/chat/codes"
)

type serverConfig struct {
	address     string
	handler     Handler
	tlsCertFile string
	tlsKeyFile  string
	logger      Logger
}

func defaultServerConfig() serverConfig {
	return serverConfig{
		address:     "localhost:4242",
		tlsCertFile: "cert.pem",
		tlsKeyFile:  "key.pem",
		logger:      NopLogger,
	}
}

// ServerOption applies option to server.
type ServerOption func(cfg *serverConfig)

// ServerOptions provides available options for server.
var ServerOptions serverOptionsNamespace

type serverOptionsNamespace struct{}

func (serverOptionsNamespace) Address(addr string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.address = addr
	}
}

func (serverOptionsNamespace) Handler(hlr Handler) ServerOption {
	return func(cfg *serverConfig) {
		cfg.handler = hlr
	}
}

func (serverOptionsNamespace) TLSCertFile(file string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.tlsCertFile = file
	}
}

func (serverOptionsNamespace) TLSKeyFile(file string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.tlsKeyFile = file
	}
}

func (serverOptionsNamespace) Logger(lgr Logger) ServerOption {
	return func(cfg *serverConfig) {
		cfg.logger = lgr
	}
}

// Server provides chat sessions.
type Server struct {
	cfg        serverConfig
	lnr        *quic.Listener
	conns      map[*quic.Conn]struct{}
	sessionsWG sync.WaitGroup

	mtx    sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a server with specified options.
func NewServer(opts ...ServerOption) *Server {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Server{
		cfg:   cfg,
		conns: make(map[*quic.Conn]struct{}),
	}
}

// Run starts the QUIC server and begins accepting incoming connections.
func (s *Server) Run() error {
	crt, err := tls.LoadX509KeyPair(s.cfg.tlsCertFile, s.cfg.tlsKeyFile)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{crt},
		NextProtos:   []string{"quic-raw"},
	}

	quicCfg := &quic.Config{}

	lnr, err := quic.ListenAddr(s.cfg.address, tlsCfg, quicCfg)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.address, err)
	}

	s.mtx.Lock()
	s.lnr = lnr
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.mtx.Unlock()

	return s.serve()
}

func closeConn(conn *quic.Conn, code codes.Code) error {
	return conn.CloseWithError(quic.ApplicationErrorCode(code), code.String())
}

func (s *Server) serve() (err error) {
	defer func() {
		if cerr := s.lnr.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close listener: %w", cerr))
		}
	}()

	for {
		conn, err := s.lnr.Accept(s.ctx)
		if err != nil {
			if errors.Is(err, quic.ErrServerClosed) {
				return nil
			}
			return errors.Join(fmt.Errorf("accept connection: %w", err), s.Stop())
		}
		lgr := s.cfg.logger.With("addr", conn.RemoteAddr().String())
		lgr.Info("connection accepted")

		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		s.mtx.Lock()
		s.conns[conn] = struct{}{}
		s.mtx.Unlock()

		s.sessionsWG.Add(1)
		go func(c *quic.Conn) {
			defer func() {
				if err := closeConn(c, codes.Done); err != nil {
					lgr.With("error", err).Error("failed to close conn")
				}
				s.mtx.Lock()
				delete(s.conns, c)
				s.mtx.Unlock()
			}()
			session, err := NewSession(s.ctx, c)
			if err != nil {
				lgr.With("error", err).Error("failed to create session")
				return
			}
			defer func() {
				if r := recover(); r != nil {
					lgr.With("panic", r).Error("panic in handler")
				}
			}()
			start := time.Now()
			s.cfg.handler(s.ctx, session)
			lgr.With("duration", time.Since(start)).Info("exit session")
		}(conn)
	}
}

// ErrServerNotRunning indicates that a server operation was attempted while the server is not running.
var ErrServerNotRunning = errors.New("server not running")

// Stop terminates the server immediately, closing all active connections.
func (s *Server) Stop() error {
	s.cancel()
	cerr := s.lnr.Close()

	s.mtx.Lock()
	conns := make([]*quic.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.conns = make(map[*quic.Conn]struct{})
	s.mtx.Unlock()

	errs := []error{cerr}
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		errs = append(errs, closeConn(conn, codes.StopServer))
	}
	return errors.Join(errs...)
}

// Shutdown gracefully stops the server, waiting for all active sessions to complete or until the given context expires.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	cerr := s.lnr.Close()

	done := make(chan struct{})
	go func() {
		s.sessionsWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	s.mtx.Lock()
	conns := make([]*quic.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.conns = make(map[*quic.Conn]struct{})
	s.mtx.Unlock()

	errs := []error{cerr}
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		errs = append(errs, closeConn(conn, codes.StopServer))
	}
	return errors.Join(errs...)
}
