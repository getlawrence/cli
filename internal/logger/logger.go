package logger

import (
	"fmt"
)

// Logf logs a formatted message using charmbracelet/log.
func Logf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Log logs a plain message with a trailing newline semantic.
func Log(msg string) {
	fmt.Println(msg)
}
