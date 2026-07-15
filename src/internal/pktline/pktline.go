// Package pktline implements Git's pkt-line framing, used by the long-running
// filter process protocol (gitattributes filter.<driver>.process).
package pktline

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// maxPayload is the largest data payload a single pkt-line may carry: the
// 65520-byte packet maximum minus the 4-byte hexadecimal length prefix.
const maxPayload = 65516

// Writer frames data into pkt-lines.
type Writer struct {
	w io.Writer
}

// NewWriter returns a Writer that frames output to w.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// WritePacket writes a single pkt-line carrying p, which must be non-empty and
// no larger than maxPayload. Use Flush for the terminating flush packet.
func (w *Writer) WritePacket(p []byte) error {
	if len(p) == 0 {
		return fmt.Errorf("pktline: empty packet; use Flush")
	}
	if len(p) > maxPayload {
		return fmt.Errorf("pktline: packet too large (%d bytes)", len(p))
	}
	if _, err := fmt.Fprintf(w.w, "%04x", len(p)+4); err != nil {
		return err
	}
	_, err := w.w.Write(p)
	return err
}

// WriteText writes s as a single pkt-line.
func (w *Writer) WriteText(s string) error { return w.WritePacket([]byte(s)) }

// WriteData splits data across as many pkt-lines as needed. Empty data writes
// nothing, leaving the caller to emit the terminating flush.
func (w *Writer) WriteData(data []byte) error {
	for len(data) > 0 {
		n := len(data)
		if n > maxPayload {
			n = maxPayload
		}
		if err := w.WritePacket(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// Flush writes a flush packet (0000).
func (w *Writer) Flush() error {
	_, err := io.WriteString(w.w, "0000")
	return err
}

// Reader parses pkt-lines.
type Reader struct {
	r *bufio.Reader
}

// NewReader returns a Reader that parses pkt-lines from r.
func NewReader(r io.Reader) *Reader { return &Reader{r: bufio.NewReader(r)} }

// ReadPacket reads the next pkt-line. flush is true for a flush packet (0000),
// in which case payload is nil.
func (r *Reader) ReadPacket() (payload []byte, flush bool, err error) {
	var header [4]byte
	if _, err := io.ReadFull(r.r, header[:]); err != nil {
		return nil, false, err
	}
	length, err := strconv.ParseUint(string(header[:]), 16, 32)
	if err != nil {
		return nil, false, fmt.Errorf("pktline: invalid length %q: %w", header, err)
	}
	if length == 0 {
		return nil, true, nil
	}
	if length < 4 || length > 65520 {
		return nil, false, fmt.Errorf("pktline: invalid packet length %d", length)
	}
	payload = make([]byte, length-4)
	if _, err := io.ReadFull(r.r, payload); err != nil {
		return nil, false, err
	}
	return payload, false, nil
}

// ReadTextLinesUntilFlush reads text pkt-lines until a flush packet, returning
// each line with a single trailing newline removed.
func (r *Reader) ReadTextLinesUntilFlush() ([]string, error) {
	var lines []string
	for {
		payload, flush, err := r.ReadPacket()
		if err != nil {
			return nil, err
		}
		if flush {
			return lines, nil
		}
		lines = append(lines, strings.TrimSuffix(string(payload), "\n"))
	}
}

// ReadDataUntilFlush concatenates data pkt-lines until a flush packet.
func (r *Reader) ReadDataUntilFlush() ([]byte, error) {
	var data []byte
	for {
		payload, flush, err := r.ReadPacket()
		if err != nil {
			return nil, err
		}
		if flush {
			return data, nil
		}
		data = append(data, payload...)
	}
}
