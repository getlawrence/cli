package types

import (
	"context"

	"github.com/getlawrence/cli/internal/domain"
)

// CodeGenerationStrategy defines the interface for different code generation approaches
type CodeGenerationStrategy interface {
	// GenerateCode generates instrumentation code for the given opportunities
	GenerateCode(ctx context.Context, opportunities []domain.Opportunity, req GenerationRequest) error

	// GetName returns the name of the strategy
	GetName() string

	// IsAvailable checks if this strategy can be used in the current environment
	IsAvailable() bool

	// GetRequiredFlags returns flags that are required for this strategy
	GetRequiredFlags() []string
}
