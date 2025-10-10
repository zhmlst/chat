// Package chat provides tools for working with the
// chat-oriented QUIC based protocol such as server, client, etc.
package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"

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

func (c *Client) handshake(ctx context.Context, conn *quic.Conn) (*quic.Stream, error) {
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		c.cfg.logger.Error(fmt.Sprintf("failed to open stream: %v", err))
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	c.cfg.logger.Debug("stream opened")

	if c.token == [16]byte{} {
		var m *msg.Message
		m, err = msg.New(stream)
		if err != nil {
			c.cfg.logger.Error(fmt.Sprintf("failed to create handshake message: %v", err))
			return nil, fmt.Errorf("failed to create handshake message: %w", err)
		}
		m.SetType(msg.TypeControl)
		c.cfg.logger.Debug("sending ack to request token")

		_, writeErr := m.Write([]byte("ack"))
		if writeErr != nil {
			c.cfg.logger.Error(fmt.Sprintf("failed to send ack: %v", writeErr))
			return nil, fmt.Errorf("failed to send ack: %w", writeErr)
		}

		var r *msg.Message
		r, err = msg.Rcv(stream)
		if err != nil {
			c.cfg.logger.Error(fmt.Sprintf("failed to receive token: %v", err))
			return nil, fmt.Errorf("failed to receive token: %w", err)
		}
		if r.Type() != msg.TypeText {
			c.cfg.logger.Error(fmt.Sprintf("unexpected message type: got %v, want TypeText", r.Type()))
			return nil, fmt.Errorf("unexpected message type: got %v, want TypeText", r.Type())
		}
		var token []byte
		token, err = r.ReadFull()
		if err != nil {
			c.cfg.logger.Error(fmt.Sprintf("failed to read full token: %v", err))
			return nil, fmt.Errorf("failed to read full token: %w", err)
		}
		copy(c.token[:], token)
		c.cfg.logger.Info("received token from server")
	}

	var m *msg.Message
	m, err = msg.New(stream)
	if err != nil {
		c.cfg.logger.Error(fmt.Sprintf("failed to create login message: %v", err))
		return nil, fmt.Errorf("failed to create login message: %w", err)
	}
	m.SetType(msg.TypeControl)
	c.cfg.logger.Debug("sending login message with token")

	_, writeErr := m.Write(append([]byte("login "), c.token[:]...))
	if writeErr != nil {
		c.cfg.logger.Error(fmt.Sprintf("failed to send login message: %v", writeErr))
		return nil, fmt.Errorf("failed to send login message: %w", writeErr)
	}

	var r *msg.Message
	r, err = msg.Rcv(stream)
	if err != nil {
		c.cfg.logger.Error(fmt.Sprintf("failed to receive login response: %v", err))
		return nil, fmt.Errorf("failed to receive login response: %w", err)
	}
	if r.Type() != msg.TypeText {
		c.cfg.logger.Error(fmt.Sprintf("unexpected response type: got %v, want TypeText", r.Type()))
		return nil, fmt.Errorf("unexpected response type: got %v, want TypeText", r.Type())
	}
	var resp []byte
	resp, err = r.ReadFull()
	if err != nil {
		c.cfg.logger.Error(fmt.Sprintf("failed to read full login response: %v", err))
		return nil, fmt.Errorf("failed to read full login response: %w", err)
	}
	if string(resp) != "ok" {
		c.cfg.logger.Warn(fmt.Sprintf("login failed, server response: %q", string(resp)))
		return nil, fmt.Errorf("login failed, server response: %q", string(resp))
	}
	c.cfg.logger.Info("login successful")

	return stream, nil
}

// TokenRepo defines a function type for storaging tokens.
type TokenRepo func(ctx context.Context, tok [16]byte) (has bool, err error)

// NopTokenRepo is a no-operation TokenRepo.
func NopTokenRepo(context.Context, [16]byte) (bool, error) { return false, nil }

func (s *Server) handshake(ctx context.Context, conn *quic.Conn, lgr Logger) (*quic.Stream, error) {
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		lgr.Error(fmt.Sprintf("failed to accept stream: %v", err))
		return nil, err
	}
	lgr.Debug("stream accepted")

	m, err := msg.Rcv(stream)
	if err != nil {
		lgr.Error(fmt.Sprintf("failed to receive initial message: %v", err))
		return nil, err
	}
	if m.Type() != msg.TypeControl {
		lgr.Warn("expected control message")
		return nil, fmt.Errorf("expected control message")
	}

	payload, err := m.ReadFull()
	if err != nil {
		lgr.Error(fmt.Sprintf("failed to read initial message: %v", err))
		return nil, err
	}

	if string(payload) == "ack" {
		lgr.Debug("client requests token")

		var token [16]byte
		if _, err = rand.Read(token[:]); err != nil {
			lgr.Error(fmt.Sprintf("failed to generate token: %v", err))
			return nil, err
		}

		var resp *msg.Message
		resp, err = msg.New(stream)
		if err != nil {
			lgr.Error(fmt.Sprintf("failed to create token message: %v", err))
			return nil, err
		}
		resp.SetType(msg.TypeText)
		if _, err = resp.Write(token[:]); err != nil {
			lgr.Error(fmt.Sprintf("failed to send token: %v", err))
			return nil, err
		}
		lgr.Info("token sent to client")

		m, err = msg.Rcv(stream)
		if err != nil {
			lgr.Error(fmt.Sprintf("failed to receive login message: %v", err))
			return nil, err
		}
		if m.Type() != msg.TypeControl {
			lgr.Warn("expected control message with token")
			return nil, fmt.Errorf("expected control message with token")
		}
		payload, err = m.ReadFull()
		if err != nil {
			lgr.Error(fmt.Sprintf("failed to read login message: %v", err))
			return nil, err
		}
		if !bytes.HasPrefix(payload, []byte("login ")) {
			lgr.Warn("expected login message")
			return nil, fmt.Errorf("expected login message")
		}
		copy(token[:], payload[6:])
		lgr.Info("received login message with token")

	} else if bytes.HasPrefix(payload, []byte("login ")) {
		var token [16]byte
		copy(token[:], payload[6:])
		var has bool
		has, err = s.cfg.tokenRepo(ctx, token)
		if err != nil {
			lgr.Error(fmt.Sprintf("token repository error: %v", err))
			return nil, err
		}
		if !has {
			lgr.Warn("invalid token")
			return nil, fmt.Errorf("invalid token")
		}
		lgr.Info("valid token received")
	} else {
		lgr.Warn(fmt.Sprintf("unexpected message: %s", string(payload)))
		return nil, fmt.Errorf("unexpected message: %s", string(payload))
	}

	resp, err := msg.New(stream)
	if err != nil {
		lgr.Error(fmt.Sprintf("failed to create response message: %v", err))
		return nil, err
	}
	resp.SetType(msg.TypeText)
	if _, err := resp.Write([]byte("ok")); err != nil {
		lgr.Error(fmt.Sprintf("failed to send ok response: %v", err))
		return nil, err
	}
	lgr.Info("handshake completed successfully")

	return stream, nil
}
