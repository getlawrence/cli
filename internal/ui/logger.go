package ui

import (
	"fmt"
	"os"
	"sync"
)

type logEntry struct {
	level   string
	message string
}

var (
	activeLogMu sync.RWMutex
	activeLogCh chan logEntry
)

// setActiveLogChannel sets the channel used by the spinner to receive log updates.
// It is intended for internal use by the spinner only.
func setActiveLogChannel(ch chan logEntry) {
	activeLogMu.Lock()
	activeLogCh = ch
	activeLogMu.Unlock()
}

// clearActiveLogChannel clears the active spinner log channel.
func clearActiveLogChannel() {
	setActiveLogChannel(nil)
}

// Logf logs a formatted message. If a spinner is active, it updates its text.
// Otherwise, it prints to stdout.
func Logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	activeLogMu.RLock()
	ch := activeLogCh
	activeLogMu.RUnlock()
    if ch != nil {
        select {
        case ch <- logEntry{level: "info", message: msg}:
        default:
            // drop if channel is full to avoid blocking
        }
        // Also mirror to stdout (captured during spinner) so logs are preserved
        // and available to callers/tests after spinner completes
        fmt.Fprint(os.Stdout, msg)
        return
    }
    fmt.Fprint(os.Stdout, msg)
}

// Log writes a plain message with newline semantics when not under a spinner.
func Log(msg string) {
	Logf("%s\n", msg)
}
