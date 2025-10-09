// Package codes defines application-level codes used to signal
// the reason for closing a QUIC connection.
package codes

import "github.com/quic-go/quic-go"

// Code represents an application-level QUIC connection close code.
// These codes indicate the reason why the server is terminating
// the connection and saying goodbye to the client.
//
//go:generate enumer -linecomment -output=enum.go -text -type=Code
type Code quic.ApplicationErrorCode

const (
	// StopServer indicates that the server is stopping and the connection
	// is closed as part of a server shutdown.
	StopServer Code = iota // stop server

	// ToManyConns indicates that the server has too many connections
	// and cannot accept more, so it closes this one.
	ToManyConns // to many connections

	// Done indicates a normal termination of the connection, i.e.,
	// the server is done interacting and closes the connection gracefully.
	Done // bye
)
