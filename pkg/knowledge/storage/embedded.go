package storage

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/logger"
)

// Embedded knowledge database
//
//go:embed knowledge.db
var embeddedKnowledgeFS embed.FS

// GetEmbeddedDatabasePath returns a path to the embedded database file.
// It extracts the embedded database to a temporary location if needed.
func GetEmbeddedDatabasePath() (string, error) {
	// Try to read the embedded database
	embeddedFile, err := embeddedKnowledgeFS.Open("knowledge.db")
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
func NewStorageWithEmbedded(dbPath string, logger logger.Logger) (*Storage, error) {
	var finalDBPath string

	// If no path provided or path doesn't exist, use embedded database
	if dbPath == "" || dbPath == ":memory:" {
		embeddedPath, err := GetEmbeddedDatabasePath()
		if err != nil {
			logger.Logf("Warning: Failed to extract embedded database, falling back to in-memory: %v\n", err)
			finalDBPath = ":memory:"
		} else {
			finalDBPath = embeddedPath
			logger.Logf("Using embedded knowledge database\n")
		}
	} else {
		// Check if the specified path exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			// File doesn't exist, try to use embedded database as fallback
			embeddedPath, embeddedErr := GetEmbeddedDatabasePath()
			if embeddedErr != nil {
				logger.Logf("Warning: Specified database not found and embedded database unavailable, using in-memory database\n")
				finalDBPath = ":memory:"
			} else {
				logger.Logf("Specified database not found, using embedded knowledge database as fallback\n")
				finalDBPath = embeddedPath
			}
		} else {
			finalDBPath = dbPath
		}
	}

	return NewStorage(finalDBPath, logger)
}

// HasEmbeddedDatabase checks if an embedded database is available
func HasEmbeddedDatabase() bool {
	file, err := embeddedKnowledgeFS.Open("knowledge.db")
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// CleanupTempDatabase removes the temporary database file if it exists
func CleanupTempDatabase() error {
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "lawrence-knowledge.db")

	if _, err := os.Stat(tempFile); err == nil {
		return os.Remove(tempFile)
	}
	return nil
}
