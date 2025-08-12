package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type UILogger struct {
	mu      sync.Mutex
	spinner *uiSpinner
}

func NewUILogger() *UILogger {
	return &UILogger{}
}

// IsTerminal reports whether stdout is attached to a terminal.
// Used to decide when to use interactive UI elements like spinners.
func IsInteractive() bool {
	// Best-effort detection without external deps: if stdout is not a character
	// device or is redirected (common in tests/CI), avoid interactive UI.
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// If it's a pipe or regular file, it's not interactive
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	return true
}

func (l *UILogger) Logf(format string, args ...interface{}) {
	l.mu.Lock()
	s := l.spinner
	l.mu.Unlock()
	if s != nil {
		text := fmt.Sprintf(format, args...)
		text = strings.TrimSuffix(text, "\n")
		text = strings.ReplaceAll(text, "\n", " ")
		s.Update(text)
		return
	}
	fmt.Printf(format, args...)
}

func (l *UILogger) Log(msg string) {
	l.mu.Lock()
	s := l.spinner
	l.mu.Unlock()
	if s != nil {
		text := strings.TrimSuffix(msg, "\n")
		text = strings.ReplaceAll(text, "\n", " ")
		s.Update(text)
		return
	}
	fmt.Println(msg)
}

// uiSpinner is a minimal spinner implementation suitable for simple CLI UIs.
// It uses a background goroutine to animate while printing to stdout.
type uiSpinner struct {
	parent  *UILogger
	mu      sync.Mutex
	text    string
	stopped chan struct{}
	failed  bool
}

func (l *UILogger) StartSpinner(text string) Spinner {
	l.mu.Lock()
	// stop previous spinner if exists
	if l.spinner != nil {
		l.spinner.internalStop(false)
		l.spinner = nil
	}
	s := &uiSpinner{parent: l, text: text, stopped: make(chan struct{})}
	l.spinner = s
	l.mu.Unlock()
	go s.loop()
	return s
}

func (s *uiSpinner) loop() {
	frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	i := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	clear := func() { fmt.Print("\r\033[2K") }
	for {
		select {
		case <-s.stopped:
			// Clear line on stop; do not render frozen frame
			clear()
			return
		case <-ticker.C:
			s.mu.Lock()
			text := s.text
			s.mu.Unlock()
			clear()
			fmt.Printf("%c %s", frames[i%len(frames)], text)
			i++
		}
	}
}

func (s *uiSpinner) Update(text string) {
	s.mu.Lock()
	s.text = text
	s.mu.Unlock()
}

func (s *uiSpinner) internalStop(failed bool) {
	s.mu.Lock()
	s.failed = failed
	s.mu.Unlock()
	select {
	case <-s.stopped:
		// already stopped
	default:
		close(s.stopped)
	}
}

func (s *uiSpinner) Stop() {
	s.internalStop(false)
	// Detach from parent
	if s.parent != nil {
		s.parent.mu.Lock()
		if s.parent.spinner == s {
			s.parent.spinner = nil
		}
		s.parent.mu.Unlock()
	}
}

func (s *uiSpinner) Fail() {
	s.internalStop(true)
	if s.parent != nil {
		s.parent.mu.Lock()
		if s.parent.spinner == s {
			s.parent.spinner = nil
		}
		s.parent.mu.Unlock()
	}
}
