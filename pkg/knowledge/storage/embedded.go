package storage

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/logger"
)

func getEmbeddedDatabasePath(dbPath string, embeddedFS embed.FS) (string, error) {
	embeddedFile, err := embeddedFS.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("embedded knowledge database not found: %w", err)
	}
	defer embeddedFile.Close()

	// Create a temporary file with a unique name to avoid conflicts
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "lawrence-knowledge.db")

	// Always extract fresh copy to ensure we have the latest embedded data
	if _, err := os.Stat(tempFile); err == nil {
		// Remove existing temp file
		os.Remove(tempFile)
	}

	// Extract the embedded database to the temporary location
	outFile, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary database file: %w", err)
	}
	defer outFile.Close()

	// Copy the embedded database content
	_, err = io.Copy(outFile, embeddedFile)
	if err != nil {
		return "", fmt.Errorf("failed to extract embedded database: %w", err)
	}

	return tempFile, nil
}

// NewStorageWithEmbedded creates a new storage instance using the embedded database
// as a fallback if no database path is provided or if the specified path doesn't exist.
func NewStorageWithEmbedded(dbPath string, embeddedFS embed.FS, logger logger.Logger) (*Storage, error) {
	var finalDBPath string

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		embeddedPath, embeddedErr := getEmbeddedDatabasePath(dbPath, embeddedFS)
		if embeddedErr != nil {
			return nil, fmt.Errorf("failed to get embedded database path: %w", embeddedErr)
		}
		logger.Logf("Using embedded knowledge database: %s\n", embeddedPath)
		finalDBPath = embeddedPath
	} else {
		logger.Logf("Using local knowledge database: %s\n", dbPath)
		finalDBPath = dbPath
	}

	return NewStorage(finalDBPath, logger)
}
