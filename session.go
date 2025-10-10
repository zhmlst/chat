package chat

import (
	"context"

	"github.com/quic-go/quic-go"
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

// NewSession accepts a new QUIC stream from the connection and returns a Session.
func NewSession(ctx context.Context, conn *quic.Conn, lgr Logger) (*Session, error) {
	lgr.Debug("accepting stream")
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	lgr.Debug("stream accepted")
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
