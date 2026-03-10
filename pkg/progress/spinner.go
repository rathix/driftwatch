package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

var frames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Spinner displays a progress spinner with a message on a terminal.
type Spinner struct {
	w       io.Writer
	mu      sync.Mutex
	msg     string
	stop    chan struct{}
	stopped chan struct{}
}

// NewSpinner creates a spinner writing to w.
// Returns a no-op spinner if w is not a terminal.
func NewSpinner(w io.Writer) *Spinner {
	if f, ok := w.(interface{ Fd() uintptr }); !ok || !term.IsTerminal(int(f.Fd())) {
		stopped := make(chan struct{})
		close(stopped)
		return &Spinner{stop: make(chan struct{}), stopped: stopped}
	}

	s := &Spinner{
		w:       w,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go s.run()
	return s
}

// Update changes the displayed message.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	select {
	case <-s.stop:
		return // already stopped
	default:
	}
	close(s.stop)
	<-s.stopped
}

func (s *Spinner) run() {
	defer close(s.stopped)
	if s.w == nil {
		return
	}

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-s.stop:
			s.clearLine()
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.msg
			s.mu.Unlock()
			if msg != "" {
				fmt.Fprintf(s.w, "\r%c %s", frames[frame%len(frames)], msg)
			}
			frame++
		}
	}
}

func (s *Spinner) clearLine() {
	if s.w != nil {
		fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", 80))
	}
}
