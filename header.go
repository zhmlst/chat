// Package chat provides tools for working with the
// chat-oriented QUIC based protocol such as server, client, etc.
package chat

// MsgType represents the type of a message in the protocol.
type MsgType byte

const (
	// MsgTypeAck represents an acknowledgment message.
	MsgTypeAck MsgType = iota
	// MsgTypeText represents a text message.
	MsgTypeText
	// MsgTypeBinary represents a binary message.
	MsgTypeBinary
	// MsgTypeControl represents a control message.
	MsgTypeControl
)

const (
	offType = 0  // offset of the message type field in the header
	offLen  = 1  // offset of the length field in the header
	offTS   = 9  // offset of the timestamp field in the header
	offID   = 25 // offset of the ID field in the header
	offTok  = 41 // offset of the token field in the header
	hdrLen  = 57 // total length of the header
)

// Header represents a fixed-size protocol header.
type Header [hdrLen]byte

// SetType sets the message type in the header.
func (h *Header) SetType(typ MsgType) {
	h[offType] = byte(typ)
}

// Type returns the message type from the header.
func (h *Header) Type() MsgType {
	return MsgType(h[offType])
}

// SetLen sets the payload length in the header.
func (h *Header) SetLen(length uint64) {
	h[offLen] = byte(length >> 56)
	h[offLen+1] = byte(length >> 48)
	h[offLen+2] = byte(length >> 40)
	h[offLen+3] = byte(length >> 32)
	h[offLen+4] = byte(length >> 24)
	h[offLen+5] = byte(length >> 16)
	h[offLen+6] = byte(length >> 8)
	h[offLen+7] = byte(length)
}

// Len returns the payload length from the header.
func (h *Header) Len() uint64 {
	return uint64(h[offLen])<<56 |
		uint64(h[offLen+1])<<48 |
		uint64(h[offLen+2])<<40 |
		uint64(h[offLen+3])<<32 |
		uint64(h[offLen+4])<<24 |
		uint64(h[offLen+5])<<16 |
		uint64(h[offLen+6])<<8 |
		uint64(h[offLen+7])
}

// SetTimestamp sets the message timestamp in the header.
func (h *Header) SetTimestamp(ts uint64) {
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
func (h *Header) Timestamp() uint64 {
	return uint64(h[offTS])<<56 |
		uint64(h[offTS+1])<<48 |
		uint64(h[offTS+2])<<40 |
		uint64(h[offTS+3])<<32 |
		uint64(h[offTS+4])<<24 |
		uint64(h[offTS+5])<<16 |
		uint64(h[offTS+6])<<8 |
		uint64(h[offTS+7])
}

// SetID sets the 16-byte message ID in the header.
func (h *Header) SetID(id [16]byte) {
	copy(h[offID:offID+len(id)], id[:])
}

// ID returns the 16-byte message ID from the header.
func (h *Header) ID() (id [16]byte) {
	return [16]byte(h[offID : offID+len(id)])
}

// SetToken sets the 16-byte message token in the header.
func (h *Header) SetToken(tok [16]byte) {
	copy(h[offTok:offTok+len(tok)], tok[:])
}

// Token returns the 16-byte message token from the header.
func (h *Header) Token() (tok [16]byte) {
	return [16]byte(h[offTok : offTok+len(tok)])
}
