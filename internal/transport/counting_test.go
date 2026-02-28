package transport

import (
	"bytes"
	"testing"
)

func TestCountingWriter_TracksBytes(t *testing.T) {
	var buf bytes.Buffer
	cw := NewCountingWriter(&buf)

	data := []byte("hello, world!")
	n, err := cw.Write(data)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes written, got %d", len(data), n)
	}
	if cw.Count() != int64(len(data)) {
		t.Fatalf("expected count %d, got %d", len(data), cw.Count())
	}

	// Write more data.
	more := []byte(" more data")
	n2, err := cw.Write(more)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	expected := int64(n + n2)
	if cw.Count() != expected {
		t.Fatalf("expected cumulative count %d, got %d", expected, cw.Count())
	}

	// Underlying buffer should have all data.
	if buf.String() != "hello, world! more data" {
		t.Fatalf("unexpected buffer content: %q", buf.String())
	}
}

func TestCountingWriter_ZeroOnInit(t *testing.T) {
	var buf bytes.Buffer
	cw := NewCountingWriter(&buf)
	if cw.Count() != 0 {
		t.Fatalf("expected initial count 0, got %d", cw.Count())
	}
}

func TestCountingWriter_EmptyWrite(t *testing.T) {
	var buf bytes.Buffer
	cw := NewCountingWriter(&buf)
	n, err := cw.Write([]byte{})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes written, got %d", n)
	}
	if cw.Count() != 0 {
		t.Fatalf("expected count 0 after empty write, got %d", cw.Count())
	}
}
