package logger

import (
	"fmt"
)

type StdoutLogger struct{}

func (l *StdoutLogger) Logf(format string, args ...interface{}) { fmt.Printf(format, args...) }
func (l *StdoutLogger) Log(msg string)                          { fmt.Println(msg) }
