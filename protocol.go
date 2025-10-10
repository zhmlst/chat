// Package chat provides tools for working with the
// chat-oriented QUIC based protocol such as server, client, etc.
package chat

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"
)

// MsgType represents the type of a message in the protocol.
type MsgType byte

const (
	// MsgTypeControl represents a control message.
	MsgTypeControl MsgType = iota
	// MsgTypeText represents a text message.
	MsgTypeText
	// MsgTypeBinary represents a binary message.
	MsgTypeBinary
)

const (
	offType = 0  // offset of the message type field in the header
	offLen  = 1  // offset of the length field in the header
	offTS   = 5  // offset of the timestamp field in the header
	offID   = 21 // offset of the ID field in the header
	offTok  = 37 // offset of the token field in the header
	hdrLen  = 53 // total length of the header
)

// header represents a fixed-size protocol header.
type header [hdrLen]byte

// SetType sets the message type in the header.
func (h *header) SetType(typ MsgType) {
	h[offType] = byte(typ)
}

// Type returns the message type from the header.
func (h *header) Type() MsgType {
	return MsgType(h[offType])
}

// SetLen sets the payload length in the header.
func (h *header) SetLen(length int32) {
	h[offLen] = byte(length >> 24)
	h[offLen+1] = byte(length >> 16)
	h[offLen+2] = byte(length >> 8)
	h[offLen+3] = byte(length)
}

// Len returns the payload length from the header.
func (h *header) Len() int32 {
	l := uint32(h[offLen])<<24 |
		uint32(h[offLen+1])<<16 |
		uint32(h[offLen+2])<<8 |
		uint32(h[offLen+3])
	return int32(l)
}

// SetTimestamp sets the message timestamp in the header.
func (h *header) SetTimestamp(ts int64) {
	h[offTS] = byte(ts >> 56)
	h[offTS+1] = byte(ts >> 48)
	h[offTS+2] = byte(ts >> 40)
	h[offTS+3] = byte(ts >> 32)
	h[offTS+4] = byte(ts >> 24)
	h[offTS+5] = byte(ts >> 16)
	h[offTS+6] = byte(ts >> 8)
	h[offTS+7] = byte(ts)
}

// Timestamp returns the message timestamp from the header.
func (h *header) Timestamp() int64 {
	return int64(h[offTS])<<56 |
		int64(h[offTS+1])<<48 |
		int64(h[offTS+2])<<40 |
		int64(h[offTS+3])<<32 |
		int64(h[offTS+4])<<24 |
		int64(h[offTS+5])<<16 |
		int64(h[offTS+6])<<8 |
		int64(h[offTS+7])
}

// SetID sets the 16-byte message ID in the header.
func (h *header) SetID(id [16]byte) {
	copy(h[offID:offID+len(id)], id[:])
}

// ID returns the 16-byte message ID from the header.
func (h *header) ID() (id [16]byte) {
	return [16]byte(h[offID : offID+len(id)])
}

// SetToken sets the 16-byte message token in the header.
func (h *header) SetToken(tok [16]byte) {
	copy(h[offTok:offTok+len(tok)], tok[:])
}

// Token returns the 16-byte message token from the header.
func (h *header) Token() (tok [16]byte) {
	return [16]byte(h[offTok : offTok+len(tok)])
}

type Message struct {
	hdr header
	pld []byte
}

var (
	ErrShortMsg = errors.New("message is too short")
)

func (m *Message) UnmarshalBinary(data []byte) error {
	if len(data) < hdrLen {
		return ErrShortMsg
	}
	copy(m.hdr[:], data[:hdrLen])
	m.pld = data[hdrLen:]
	return nil
}

func (m *Message) MarshalBinary() ([]byte, error) {
	data := make([]byte, hdrLen+len(m.pld))
	copy(data[:hdrLen], m.hdr[:])
	copy(data[hdrLen:], m.pld)
	return data, nil
}

func (m *Message) Read(r io.Reader) error {
	n, err := io.ReadFull(r, m.hdr[:])
	if err != nil {
		return err
	}
	if n < hdrLen {
		return fmt.Errorf("%w: %d/%d", ErrShortMsg, n, hdrLen)
	}

	payloadLen := m.hdr.Len()
	if payloadLen < 0 {
		return fmt.Errorf("invalid payload length: %d", payloadLen)
	}

	m.pld = make([]byte, payloadLen)
	for total := 0; total < int(payloadLen); {
		n, err := r.Read(m.pld[total:])
		if err != nil {
			return err
		}
		total += n
	}

	m.hdr.SetLen(int32(len(m.pld)))
	return nil
}

func (m *Message) Write(w io.Writer) error {
	n, err := w.Write(m.hdr[:])
	if err != nil {
		return err
	}
	if n < hdrLen {
		return fmt.Errorf("%w: %d/%d", io.ErrShortWrite, n, hdrLen)
	}

	for total := 0; int32(total) < m.hdr.Len(); {
		n, err := w.Write(m.pld[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func (m *Message) SetType(typ MsgType) {
	m.hdr.SetType(typ)
}

func (m *Message) Type() MsgType {
	return m.hdr.Type()
}

func (m *Message) Len() int {
	return int(m.hdr.Len())
}

func (m *Message) SetTimestamp(ts time.Time) {
	m.hdr.SetTimestamp(ts.UnixMilli())
}

func (m *Message) Timestamp() time.Time {
	ms := m.hdr.Timestamp()
	sec := int64(ms / 1000)
	nsec := int64(ms % 1000 * 1_000_000)
	return time.Unix(sec, nsec)
}

func (m *Message) EnsureID() error {
	var zero [16]byte
	if m.hdr.ID() == zero {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return err
		}
		m.hdr.SetID(id)
	}
	return nil
}

func (m *Message) ID() [16]byte {
	return m.hdr.ID()
}

func (m *Message) SetToken(tok [16]byte) {
	m.hdr.SetToken(tok)
}

func (m *Message) Token() [16]byte {
	return m.hdr.Token()
}
