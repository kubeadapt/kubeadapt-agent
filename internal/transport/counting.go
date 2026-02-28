package transport

import (
	"io"
	"sync/atomic"
)

// CountingWriter wraps an io.Writer and counts bytes written through it.
// It is safe for concurrent reads of the count while writes are in progress.
type CountingWriter struct {
	w     io.Writer
	count int64
}

// NewCountingWriter returns a CountingWriter wrapping w.
func NewCountingWriter(w io.Writer) *CountingWriter {
	return &CountingWriter{w: w}
}

// Write writes p to the underlying writer and adds the written byte count.
func (cw *CountingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	atomic.AddInt64(&cw.count, int64(n))
	return n, err
}

// Count returns the total number of bytes written so far.
func (cw *CountingWriter) Count() int64 {
	return atomic.LoadInt64(&cw.count)
}
