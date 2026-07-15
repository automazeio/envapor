package pktline

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteText("version=2\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte("x"), maxPayload*2+5)
	if err := w.WriteData(big); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r := NewReader(&buf)
	lines, err := r.ReadTextLinesUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "version=2" {
		t.Fatalf("lines = %v, want [version=2]", lines)
	}
	data, err := r.ReadDataUntilFlush()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, big) {
		t.Fatalf("data round trip mismatch: got %d bytes want %d", len(data), len(big))
	}
}

func TestWriteDataChunksLargeInput(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteData(bytes.Repeat([]byte("y"), maxPayload+1)); err != nil {
		t.Fatal(err)
	}
	// A full packet is maxPayload+4 = 0xfff0 bytes; the remainder is a 5-byte
	// (0x0005) packet.
	if !strings.HasPrefix(buf.String(), "fff0") {
		t.Fatalf("first packet header = %q, want fff0", buf.String()[:4])
	}
}

func TestReadRejectsInvalidLength(t *testing.T) {
	r := NewReader(strings.NewReader("zzzz"))
	if _, _, err := r.ReadPacket(); err == nil {
		t.Fatal("expected error for non-hex length prefix")
	}
}
