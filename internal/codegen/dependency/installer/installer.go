package installer

import (
	"context"
)

// Installer installs requested dependencies for a language/ecosystem
type Installer interface {
	Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error
}
