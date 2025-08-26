package cmd

import (
	"embed"

	"github.com/getlawrence/cli/internal/logger"
)

// AppConfig holds all the shared configuration and dependencies
type AppConfig struct {
	EmbeddedDB embed.FS
	Logger     logger.Logger
}

// NewAppConfig creates a new configuration instance
func NewAppConfig(embeddedDB embed.FS, logger logger.Logger) *AppConfig {
	return &AppConfig{
		EmbeddedDB: embeddedDB,
		Logger:     logger,
	}
}
