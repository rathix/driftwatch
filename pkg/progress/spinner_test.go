package progress

import (
	"bytes"
	"testing"
	"time"
)

func TestSpinner_NoopOnNonTTY(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)
	s.Update("hello")
	time.Sleep(150 * time.Millisecond)
	s.Stop()

	// Non-TTY writer: spinner should not output anything
	if buf.Len() != 0 {
		t.Errorf("expected no output for non-TTY, got %d bytes", buf.Len())
	}
}

func TestSpinner_StopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)
	s.Stop()
	s.Stop() // should not panic
}

func TestSpinner_UpdateDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)
	s.Update("testing")
	s.Update("another message")
	s.Stop()
}
