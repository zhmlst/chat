// Package chat provides tools for working with the
// chat-oriented QUIC based protocol such as server, client, etc.
package chat

import (
	"context"
	"io"
)

func (s *Server) handshake(ctx context.Context, rw io.ReadWriter) error {
	return nil
}

func (c *Client) handshake(ctx context.Context, rw io.ReadWriter) error {
	return nil
}
