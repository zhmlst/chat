package chat

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chzyer/readline"
	"github.com/quic-go/quic-go"
)

type clientConfig struct {
	servers []string
	certs   []string
	insec   bool
	logger  Logger
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		servers: []string{"localhost:4242"},
		certs:   []string{"cert.pem"},
		logger:  NopLogger,
	}
}

// ClientOption applies option to client.
type ClientOption func(cfg *clientConfig)

// ClientOptions provides available options for client.
var ClientOptions clientOptionsNamespace

type clientOptionsNamespace struct{}

func (clientOptionsNamespace) Servers(addrs []string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.servers = addrs
	}
}

func (clientOptionsNamespace) Certs(files []string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.certs = files
	}
}

func (clientOptionsNamespace) Insec(insec bool) ClientOption {
	return func(cfg *clientConfig) {
		cfg.insec = insec
	}
}

func (clientOptionsNamespace) Logger(lgr Logger) ClientOption {
	return func(cfg *clientConfig) {
		cfg.logger = lgr
	}
}

// Client is a QUIC chat client.
type Client struct {
	cfg   clientConfig
	token [16]byte
}

// NewClient creates a client with specified options.
func NewClient(opts ...ClientOption) *Client {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Client{
		cfg: cfg,
	}
}

// Dial connects the client to a server and starts the chat loop.
func (c *Client) Dial(ctx context.Context) error {
	crts, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("get system certs: %w", err)
	}

	for _, certfile := range c.cfg.certs {
		var crt []byte
		crt, err = os.ReadFile(certfile)
		if err != nil {
			c.cfg.logger.With("error", err).Error("failed to read cert")
			continue
		}
		if !crts.AppendCertsFromPEM(crt) {
			c.cfg.logger.With("file", certfile).Warn("failed to append cert")
		}
	}

	tlsCfg := &tls.Config{
		RootCAs:            crts,
		InsecureSkipVerify: c.cfg.insec,
		NextProtos:         []string{"quic-raw"},
	}

	quicCfg := &quic.Config{
		KeepAlivePeriod: 20 * time.Second,
	}

	var conn *quic.Conn
	for _, addr := range c.cfg.servers {
		conn, err = quic.DialAddr(ctx, addr, tlsCfg, quicCfg)
		if err != nil {
			c.cfg.logger.With("error", err).Error(fmt.Sprintf("failed to dial %s", addr))
			continue
		}
		break
	}
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	return c.handleConn(ctx, conn)
}

func (c *Client) handleConn(ctx context.Context, conn *quic.Conn) error {
	stream, err := c.handshake(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed handshake: %w", err)
	}
	defer stream.Close()

	rl, err := readline.New("> ")
	if err != nil {
		return fmt.Errorf("create readline: %w", err)
	}
	defer rl.Close()

	errCh := make(chan error, 2)

	go func() {
		for {
			input, err := rl.ReadSlice()
			if err != nil {
				if err == readline.ErrInterrupt || err == io.EOF {
					errCh <- nil
				} else {
					errCh <- fmt.Errorf("read input: %w", err)
				}
				return
			}

			_, err = stream.Write(input)
			if err != nil {
				errCh <- fmt.Errorf("write to stream: %w", err)
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, buflen)
		for {
			n, err := stream.Read(buf)
			if err != nil {
				if err == io.EOF {
					errCh <- nil
				} else {
					errCh <- fmt.Errorf("read from stream: %w", err)
				}
				return
			}

			fmt.Println("\r" + string(buf[:n]))
			rl.Refresh()
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
