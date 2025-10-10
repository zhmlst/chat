// Package chat provides tools for working with the
// chat-oriented QUIC based protocol such as server, client, etc.
package chat

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/quic-go/quic-go"
	"github.com/zhmlst/chat/internal/msg"
)

const (
	buflen = 4096
	chansz = 8
)

// Session represents a QUIC session stream.
type Session struct {
	stream *quic.Stream
	lgr    Logger
}

// NewSession a new chat session.
func NewSession(stream *quic.Stream, lgr Logger) (*Session, error) {
	return &Session{
		stream: stream,
		lgr:    lgr,
	}, nil
}

// Input returns a channel that receives incoming data from the session stream.
func (s *Session) Input(ctx context.Context) <-chan []byte {
	ch := make(chan []byte, chansz)
	buf := make([]byte, buflen)
	go func() {
		defer close(ch)
		for {
			n, err := s.stream.Read(buf)
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-s.stream.Context().Done():
				return
			case ch <- append([]byte(nil), buf[:n]...):
			}
		}
	}()
	return ch
}

// Output returns a channel where writing to it sends data to the session stream.
func (s *Session) Output(ctx context.Context) chan<- []byte {
	ch := make(chan []byte, chansz)
	go func() {
		defer func() { _ = s.stream.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stream.Context().Done():
				return
			case buf, ok := <-ch:
				if !ok {
					return
				}
				if _, err := s.stream.Write(buf); err != nil {
					return
				}
			}
		}
	}()
	return ch
}

// Handler defines a function type for handling sessions.
type Handler func(ctx context.Context, s *Session)

var (
	// ErrInvalidToken is returned when a token received from a client
	// does not match the expected size or format.
	ErrInvalidToken = errors.New("invalid token")

	// ErrInternal is returned when an unexpected internal server error occurs,
	// such as failures in the handshake process or token handling.
	ErrInternal = errors.New("internal server error")
)

func (c *Client) token(stream *quic.Stream, rep bool) (tok [16]byte, err error) {
	lgr := c.cfg.logger.With("op", "token")
	rawtok, err := os.ReadFile(c.cfg.token)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return tok, fmt.Errorf("failed to read token file: %w", err)
	}
	if len(rawtok) != cap(tok) || rep {
		lgr.With("rep", rep).Debug("requesting new token")
		m, err := msg.New(stream)
		if err != nil {
			return tok, fmt.Errorf("failed to create message: %w", err)
		}
		m.SetType(msg.TypeControl)
		if _, err = m.Write([]byte("ack")); err != nil {
			return tok, fmt.Errorf("failed to write message: %w", err)
		}
		r, err := msg.Rcv(stream)
		if err != nil {
			return tok, fmt.Errorf("failed to receive message: %w", err)
		}
		rawtok, err = r.ReadFull()
		if err != nil {
			return tok, fmt.Errorf("failed to read message: %w", err)
		}
		if len(rawtok) != cap(tok) {
			return tok, fmt.Errorf("%w: %s", ErrInvalidToken, string(rawtok))
		}
		lgr.Debug("received new token, saving")
		if err := c.saveToken([16]byte(rawtok)); err != nil {
			return tok, err
		}
		lgr.Info("new token saved")
	} else {
		lgr.Debug("using existing token")
	}
	return [16]byte(rawtok), nil
}

func (c *Client) saveToken(tok [16]byte) (err error) {
	lgr := c.cfg.logger.With("module", "saveToken")
	dir := filepath.Dir(c.cfg.token)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to mkdir %s for token file: %w", dir, err)
	}
	file, err := os.OpenFile(c.cfg.token, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("failed to open token file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close token file: %w", cerr))
		}
	}()
	if _, err = file.Write(tok[:]); err != nil {
		return fmt.Errorf("failed to save token file %s: %w", c.cfg.token, err)
	}
	lgr.Info("token file written")
	return nil
}

func (c *Client) handshake(ctx context.Context, conn *quic.Conn) (stream *quic.Stream, err error) {
	lgr := c.cfg.logger.With("module", "handshake", "addr", conn.RemoteAddr().String())
	lgr.Info("starting handshake")

	stream, err = conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	lgr.Debug("stream opened")
	// close stream on handshake failure
	defer func() {
		if err != nil {
			if cerr := stream.Close(); cerr != nil {
				err = errors.Join(err, fmt.Errorf("failed to close stream: %w", cerr))
			}
		}
	}()

	attempt, maxAttempts := 1, 3
tok:
	tok, err := c.token(stream, attempt > 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	lgr.With("attempt", attempt).Debug("token obtained")

	m, err := msg.New(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}
	m.SetType(msg.TypeControl)
	m.SetToken(tok)
	if _, err = m.Write([]byte("login")); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}
	lgr.With("attempt", attempt).Debug("login message sent")

	r, err := msg.Rcv(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to receive message: %w", err)
	}
	resp, err := r.ReadFull()
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	if string(resp) != "ok" {
		lgr.With("attempt", attempt).Warn("login response not ok, retrying")
		if attempt > maxAttempts {
			return nil, ErrInternal
		}
		attempt++
		goto tok
	}

	lgr.With("attempt", attempt).Info("handshake completed successfully")
	return stream, nil
}

func (s *Server) handshake(ctx context.Context, conn *quic.Conn) (stream *quic.Stream, err error) {
	lgr := s.cfg.logger.With("addr", conn.RemoteAddr().String(), "op", "handshake")
	lgr.Debug("accepting stream")

	stream, err = conn.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}
	defer func() {
		if err != nil {
			if cerr := stream.Close(); cerr != nil {
				err = errors.Join(err, fmt.Errorf("failed to close stream: %w", cerr))
			}
		}
	}()

rcv:
	r, err := msg.Rcv(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to receive message: %w", err)
	}
	lgr.Debug("message received")

	pld, err := r.ReadFull()
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	switch string(pld) {
	case "ack":
		l := lgr.With("phase", "ack")
		l.Debug("processing ack")
		var tok [16]byte
		if _, err = rand.Read(tok[:]); err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}
		if err = s.cfg.tokenRepo.SaveToken(ctx, tok); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
		l.Info("generated and saved token")

		m, err := msg.New(stream)
		if err != nil {
			return nil, fmt.Errorf("failed to create token message: %w", err)
		}
		m.SetType(msg.TypeControl)
		if _, err = m.Write(tok[:]); err != nil {
			return nil, fmt.Errorf("failed to send token: %w", err)
		}
		l.Debug("token sent")

	case "login":
		l := lgr.With("phase", "login")
		l.Debug("processing login")
		tok := r.Token()
		has, err := s.cfg.tokenRepo.HasToken(ctx, tok)
		if err != nil {
			return nil, fmt.Errorf("failed to check token: %w", err)
		}

		m, err := msg.New(stream)
		if err != nil {
			return nil, fmt.Errorf("failed to create response message: %w", err)
		}
		m.SetType(msg.TypeControl)

		if !has {
			if _, err = m.Write([]byte("no")); err != nil {
				return nil, fmt.Errorf("failed to write response: %w", err)
			}
			l.Warn("unknown token, asking client to retry")
			goto rcv
		}

		if _, err = m.Write([]byte("ok")); err != nil {
			return nil, fmt.Errorf("failed to write response: %w", err)
		}
		l.Info("client authenticated")
		return stream, nil

	default:
		l := lgr.With("phase", "unknown")
		l.Warn("unknown message type, responding no")
		m, err := msg.New(stream)
		if err != nil {
			return nil, fmt.Errorf("failed to create response message: %w", err)
		}
		m.SetType(msg.TypeControl)
		if _, err = m.Write([]byte("no")); err != nil {
			return nil, fmt.Errorf("failed to write response: %w", err)
		}
	}
	goto rcv
}
