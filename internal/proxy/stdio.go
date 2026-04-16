package proxy

import (
	"bufio"
	"io"
)

// Scanner reads newline-delimited JSON-RPC messages from a reader.
type Scanner struct {
	scanner *bufio.Scanner
}

// NewScanner creates a scanner for reading JSON-RPC messages.
// Supports messages up to 10MB for large streaming responses.
func NewScanner(r io.Reader) *Scanner {
	s := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 10*1024*1024)
	return &Scanner{scanner: s}
}

// Scan reads the next message. Returns false when no more messages are available.
func (s *Scanner) Scan() bool {
	return s.scanner.Scan()
}

// Bytes returns the current message bytes.
func (s *Scanner) Bytes() []byte {
	return s.scanner.Bytes()
}

// Err returns any error encountered during scanning.
func (s *Scanner) Err() error {
	return s.scanner.Err()
}

// WriteMessage writes a message followed by a newline to the writer.
func WriteMessage(w io.Writer, msg []byte) error {
	data := make([]byte, len(msg)+1)
	copy(data, msg)
	data[len(msg)] = '\n'
	_, err := w.Write(data)
	return err
}
