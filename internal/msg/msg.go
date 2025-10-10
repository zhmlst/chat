package msg

import (
	"crypto/rand"
	"fmt"
	"io"
	"iter"
	"time"
)

type Type byte

const (
	TypeControl Type = iota
	TypeText
	TypeBinary
)

const (
	offType = 0
	offLen  = 1
	offTS   = 5
	offID   = 21
	offTok  = 37
	hdrLen  = 53
)

const (
	buflen = 4096
)

type Message struct {
	hdr [hdrLen]byte
	r   io.Reader
	w   io.Writer
}

func New(w io.Writer) (*Message, error) {
	m := &Message{w: w}
	var id [16]byte
	_, err := rand.Read(id[:])
	if err != nil {
		return nil, fmt.Errorf("msg id gen: %w", err)
	}
	m.setID(id)
	m.setTimestamp(time.Now().UTC())
	return m, nil
}

func writeFull(w io.Writer, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := w.Write(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (m *Message) Write(pld []byte) (int, error) {
	m.SetLen(uint32(len(pld)))
	nHdr, err := writeFull(m.w, m.hdr[:])
	if err != nil {
		return nHdr, err
	}
	nPld, err := writeFull(m.w, pld)
	return nHdr + nPld, err
}

func Rcv(r io.Reader) (*Message, error) {
	m := &Message{r: r}
	for total := 0; total < hdrLen; {
		n, err := r.Read(m.hdr[total:])
		if err != nil {
			return nil, err
		}
		total += n
	}
	return m, nil
}

func (m *Message) Read() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		buf := make([]byte, buflen)
		for total := 0; total < m.Len(); {
			if total+len(buf) > m.Len() {
				buf = buf[:m.Len()-total]
			}
			n, err := m.r.Read(buf)
			if err == io.EOF {
				return
			}
			if !yield(append([]byte(nil),
				buf[:n]...), err) {
				return
			}
			total += n
		}
	}
}

func (m *Message) setID(id [16]byte) {
	copy(m.hdr[offID:offID+len(id)], id[:])
}

func (m *Message) ID() [16]byte {
	return [16]byte(m.hdr[offID : offID+16])
}

func (m *Message) SetToken(tok [16]byte) {
	copy(m.hdr[offTok:offTok+len(tok)], tok[:])
}

func (m *Message) Token() [16]byte {
	return [16]byte(m.hdr[offTok : offTok+16])
}

func (m *Message) SetType(typ Type) {
	m.hdr[offType] = byte(typ)
}

func (m *Message) Type() Type {
	return Type(m.hdr[offType])
}

func (m *Message) SetLen(length uint32) {
	m.hdr[offLen] = byte(length >> 24)
	m.hdr[offLen+1] = byte(length >> 16)
	m.hdr[offLen+2] = byte(length >> 8)
	m.hdr[offLen+3] = byte(length)
}

func (m *Message) Len() int {
	return int(uint32(m.hdr[offLen])<<24 |
		uint32(m.hdr[offLen+1])<<16 |
		uint32(m.hdr[offLen+2])<<8 |
		uint32(m.hdr[offLen+3]),
	)
}

func (m *Message) setTimestamp(ts time.Time) {
	ms := uint64(ts.UnixMilli())
	for i := range 8 {
		m.hdr[offTS+i] = byte(ms >> (56 - 8*i))
	}
}

func (m *Message) Timestamp() time.Time {
	var ms uint64
	for i := range 8 {
		ms = (ms << 8) | uint64(m.hdr[offTS+i])
	}
	sec := int64(ms / 1000)
	nsec := int64(ms%1000) * 1_000_000
	return time.Unix(sec, nsec)
}
