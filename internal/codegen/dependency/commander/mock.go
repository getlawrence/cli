package commander

import (
	"context"
	"fmt"
	"strings"
)

// Mock implements Commander for testing
type Mock struct {
	Commands      map[string]bool   // which commands exist
	Responses     map[string]string // command pattern -> output
	Errors        map[string]error  // command pattern -> error
	RecordedCalls []RecordedCall    // all calls made
}

// RecordedCall captures a command invocation
type RecordedCall struct {
	Name string
	Args []string
	Dir  string
}

// NewMock creates a mock commander
func NewMock() *Mock {
	return &Mock{
		Commands:  make(map[string]bool),
		Responses: make(map[string]string),
		Errors:    make(map[string]error),
	}
}

// LookPath checks if a command exists in the mock
func (m *Mock) LookPath(name string) (string, error) {
	if m.Commands[name] {
		return "/usr/bin/" + name, nil
	}
	return "", fmt.Errorf("exec: %q: executable file not found in $PATH", name)
}

// Run records the call and returns mocked response
func (m *Mock) Run(ctx context.Context, name string, args []string, dir string) (string, error) {
	m.RecordedCalls = append(m.RecordedCalls, RecordedCall{
		Name: name,
		Args: args,
		Dir:  dir,
	})

	// Build command key for lookup
	key := name + " " + strings.Join(args, " ")

	// Check for exact match first
	if err, ok := m.Errors[key]; ok {
		return "", err
	}
	if resp, ok := m.Responses[key]; ok {
		return resp, nil
	}

	// Check for prefix match
	for pattern, err := range m.Errors {
		if strings.HasPrefix(key, pattern) {
			return "", err
		}
	}
	for pattern, resp := range m.Responses {
		if strings.HasPrefix(key, pattern) {
			return resp, nil
		}
	}

	// Default response
	return "", nil
}
