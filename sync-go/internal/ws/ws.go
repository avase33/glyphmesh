// Package ws is a minimal, dependency-free RFC 6455 WebSocket server — the
// "efficient bitwise serialization protocol" the architecture calls for,
// implemented against Go's net/http hijack interface with no third-party code.
//
// It supports the subset a collaborative editor needs: single-fragment text
// frames of any length, ping/pong, and close. Client frames are masked (per the
// spec); server frames are not.
package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Opcodes.
const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

// AcceptKey computes the Sec-WebSocket-Accept response value for a client key.
func AcceptKey(clientKey string) string {
	h := sha1.New()
	io.WriteString(h, clientKey+wsGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Conn is an upgraded WebSocket connection.
type Conn struct {
	conn net.Conn
	br   *bufio.Reader
	bw   *bufio.Writer
}

// Upgrade performs the handshake and hijacks the connection.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("not a websocket upgrade")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer does not support hijack")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + AcceptKey(key) + "\r\n\r\n"
	if _, err := rw.WriteString(resp); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	return &Conn{conn: conn, br: rw.Reader, bw: rw.Writer}, nil
}

// ReadMessage returns the next text message payload, transparently answering
// pings and surfacing io.EOF on close.
func (c *Conn) ReadMessage() ([]byte, error) {
	for {
		op, payload, err := readFrame(c.br)
		if err != nil {
			return nil, err
		}
		switch op {
		case opText:
			return payload, nil
		case opPing:
			_ = writeFrame(c.bw, opPong, payload)
		case opClose:
			return nil, io.EOF
		case opPong:
			// ignore
		default:
			// ignore binary/continuation for this app
		}
	}
}

// WriteText sends a single unfragmented text frame.
func (c *Conn) WriteText(b []byte) error {
	return writeFrame(c.bw, opText, b)
}

// Close sends a close frame and tears down the socket.
func (c *Conn) Close() error {
	_ = writeFrame(c.bw, opClose, nil)
	return c.conn.Close()
}

// readFrame parses one frame. Returns opcode and unmasked payload.
func readFrame(br *bufio.Reader) (byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := hdr[0] & 0x0f
	masked := hdr[1]&0x80 != 0
	length := uint64(hdr[1] & 0x7f)

	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext[:])
	}

	if length > 16<<20 {
		return 0, nil, fmt.Errorf("frame too large: %d", length)
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(br, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(br, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i&3]
		}
	}
	return op, payload, nil
}

// writeFrame writes one unmasked, unfragmented frame.
func writeFrame(bw *bufio.Writer, op byte, payload []byte) error {
	var hdr []byte
	b0 := byte(0x80) | op // FIN + opcode
	n := len(payload)
	switch {
	case n < 126:
		hdr = []byte{b0, byte(n)}
	case n < 1<<16:
		hdr = []byte{b0, 126, byte(n >> 8), byte(n)}
	default:
		hdr = make([]byte, 10)
		hdr[0] = b0
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:], uint64(n))
	}
	if _, err := bw.Write(hdr); err != nil {
		return err
	}
	if _, err := bw.Write(payload); err != nil {
		return err
	}
	return bw.Flush()
}
