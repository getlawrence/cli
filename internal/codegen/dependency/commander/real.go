package commander

import (
	"context"
	"os/exec"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// Real implements Commander using actual system commands
type Real struct{}

// NewReal creates a real commander
func NewReal() types.Commander {
	return &Real{}
}

// LookPath checks if a command exists
func (r *Real) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// Run executes a command
func (r *Real) Run(ctx context.Context, name string, args []string, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}
