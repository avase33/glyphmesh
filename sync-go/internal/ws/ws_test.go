package ws

import (
	"bufio"
	"bytes"
	"testing"
)

// The canonical example from RFC 6455 §1.3.
func TestAcceptKeyRFCExample(t *testing.T) {
	got := AcceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("AcceptKey = %q, want %q", got, want)
	}
}

func buildMaskedFrame(op byte, payload []byte) []byte {
	mask := [4]byte{0x11, 0x22, 0x33, 0x44}
	var buf bytes.Buffer
	buf.WriteByte(0x80 | op)
	n := len(payload)
	if n < 126 {
		buf.WriteByte(0x80 | byte(n))
	} else {
		buf.WriteByte(0x80 | 126)
		buf.WriteByte(byte(n >> 8))
		buf.WriteByte(byte(n))
	}
	buf.Write(mask[:])
	for i, b := range payload {
		buf.WriteByte(b ^ mask[i&3])
	}
	return buf.Bytes()
}

func TestReadMaskedTextFrame(t *testing.T) {
	frame := buildMaskedFrame(opText, []byte("hi there"))
	op, payload, err := readFrame(bufio.NewReader(bytes.NewReader(frame)))
	if err != nil {
		t.Fatal(err)
	}
	if op != opText {
		t.Fatalf("op = %d", op)
	}
	if string(payload) != "hi there" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestReadExtendedLength(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 300)
	frame := buildMaskedFrame(opText, big)
	_, payload, err := readFrame(bufio.NewReader(bytes.NewReader(frame)))
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != 300 {
		t.Fatalf("len = %d", len(payload))
	}
}

func TestWriteFrameHeaderUnmasked(t *testing.T) {
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)
	if err := writeFrame(bw, opText, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	if out[0] != 0x81 { // FIN + text
		t.Fatalf("byte0 = %#x", out[0])
	}
	if out[1] != 0x03 { // unmasked, len 3
		t.Fatalf("byte1 = %#x", out[1])
	}
	if string(out[2:]) != "abc" {
		t.Fatalf("payload = %q", out[2:])
	}
}

func TestWriteReadRoundTripLarge(t *testing.T) {
	// server writes (unmasked); we read it back with a mask-agnostic reader by
	// wrapping it as an unmasked frame the reader also accepts.
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)
	msg := bytes.Repeat([]byte("z"), 500)
	if err := writeFrame(bw, opText, msg); err != nil {
		t.Fatal(err)
	}
	op, payload, err := readFrame(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if op != opText || len(payload) != 500 {
		t.Fatalf("op=%d len=%d", op, len(payload))
	}
}
