package logger

type Logger interface {
	Logf(format string, args ...interface{})
	Log(msg string)
}

// Spinner displays progress for a long-running operation.
// Implementations should be safe for single-threaded Start/Stop/Fail usage.
type Spinner interface {
	// Update changes the spinner text while running.
	Update(text string)
	// Stop stops the spinner and prints a success indicator.
	Stop()
	// Fail stops the spinner and prints a failure indicator.
	Fail()
}

// noOpSpinner is used when output is non-interactive (e.g., tests, piped output).
// It performs no rendering to keep output stable.
type noOpSpinner struct{}

func (n *noOpSpinner) Update(text string) {}
func (n *noOpSpinner) Stop()              {}
func (n *noOpSpinner) Fail()              {}
