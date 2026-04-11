package cli

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// spinner displays an animated progress indicator.
type spinner struct {
	writer  io.Writer
	message string
	done    chan struct{}
	stopped sync.Once
}

// newSpinner creates and starts a spinner with the given message.
func newSpinner(w io.Writer, message string) *spinner {
	s := &spinner{
		writer:  w,
		message: message,
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *spinner) run() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			// Clear the spinner line
			fmt.Fprintf(s.writer, "\r\033[K")
			return
		case <-ticker.C:
			fmt.Fprintf(s.writer, "\r  %s %s", frames[i%len(frames)], s.message)
			i++
		}
	}
}

// stop stops the spinner and clears the line.
func (s *spinner) stop() {
	s.stopped.Do(func() {
		close(s.done)
	})
}
