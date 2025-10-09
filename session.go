package chat

import (
	"context"

	"github.com/quic-go/quic-go"
)

// Session ...
type Session struct {
	conn *quic.Conn
}

// NewSession ...
func NewSession(conn *quic.Conn) *Session {
	return &Session{
		conn: conn,
	}
}

// Input ...
func (s *Session) Input() (<-chan []byte, error) { return nil, nil }

// Output ...
func (s *Session) Output() (chan<- []byte, error) { return nil, nil }

// Handler ...
type Handler func(ctx context.Context, s *Session)
