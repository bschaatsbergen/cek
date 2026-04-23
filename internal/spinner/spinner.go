package spinner

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Spinner represents a terminal UI spinner.
type Spinner struct {
	message string
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
}

// New creates a new Spinner with the given message.
func New(message string) *Spinner {
	return &Spinner{
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner in a separate goroutine.
func (s *Spinner) Start() {
	go func() {
		// only after stopping the spinner we close the done channel
		// to signal that the spinner has finished cleaning up.
		defer close(s.done)

		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0

		for {
			select {
			case <-s.stop:
				// the \r\033[k sequence is an ANSI escape code that deletes
				// everything from the cursor to the end of the line.
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-time.After(100 * time.Millisecond):
				fmt.Fprintf(os.Stderr, "\r\033[K%s %s", frames[i%len(frames)], s.message)
				i++
			}
		}
	}()
}

// Stop stops the spinner and waits for it to finish cleaning-up the stderr output.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stop:
	default:
		close(s.stop)
	}

	// Wait for the spinner to finish cleaning up the stderr.
	<-s.done
}
