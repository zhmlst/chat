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
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		servers: []string{"localhost:4242"},
		certs:   []string{"cert.pem"},
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

// Client is a QUIC chat client.
type Client struct {
	cfg clientConfig
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
			// TODO: logging
			continue
		}
		if !crts.AppendCertsFromPEM(crt) {
			// TODO: logging
			_ = crt
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
			// TODO: logging
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
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	rl, err := readline.New("> ")
	if err != nil {
		return fmt.Errorf("create readline: %w", err)
	}

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
		buf := make([]byte, 4096)
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

			fmt.Print(string(buf[:n]))
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
