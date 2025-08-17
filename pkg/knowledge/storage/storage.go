package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/types"
	_ "github.com/mattn/go-sqlite3"
)

// Storage represents the knowledge base storage interface using SQLite
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new storage instance with SQLite database
func NewStorage(dbPath string) (*Storage, error) {
	// Handle empty path by creating an in-memory database
	if dbPath == "" {
		dbPath = ":memory:"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize the database schema
	if err := initDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &Storage{db: db}, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// SaveKnowledgeBase saves the knowledge base to the SQLite database using parallel processing
func (s *Storage) SaveKnowledgeBase(kb *types.KnowledgeBase, filename string) error {
	startTime := time.Now()

	// For small datasets, use sequential processing
	if len(kb.Components) < 10 {
		err := s.saveKnowledgeBaseSequential(kb)
		if err == nil {
			fmt.Printf("Sequential processing completed in %v for %d components\n", time.Since(startTime), len(kb.Components))
		}
		return err
	}

	// Start a transaction for better performance
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing data
	if _, err := tx.Exec("DELETE FROM versions"); err != nil {
		return fmt.Errorf("failed to clear versions: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM components"); err != nil {
		return fmt.Errorf("failed to clear components: %w", err)
	}

	// Use parallel processing for better performance
	err = s.saveKnowledgeBaseParallel(tx, kb)
	if err == nil {
		fmt.Printf("Parallel processing completed in %v for %d components using %d workers\n",
			time.Since(startTime), len(kb.Components), runtime.NumCPU())
	}
	return err
}

// saveKnowledgeBaseSequential is the original sequential implementation for small datasets
func (s *Storage) saveKnowledgeBaseSequential(kb *types.KnowledgeBase) error {
	// Start a transaction for better performance
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing data
	if _, err := tx.Exec("DELETE FROM versions"); err != nil {
		return fmt.Errorf("failed to clear versions: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM components"); err != nil {
		return fmt.Errorf("failed to clear components: %w", err)
	}

	// Insert components sequentially
	for _, component := range kb.Components {
		componentID, err := s.insertComponent(tx, &component)
		if err != nil {
			return fmt.Errorf("failed to insert component %s: %w", component.Name, err)
		}

		// Insert versions for this component
		for _, version := range component.Versions {
			if err := s.insertVersion(tx, componentID, &version); err != nil {
				return fmt.Errorf("failed to insert version %s for component %s: %w", version.Name, component.Name, err)
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// saveKnowledgeBaseParallel processes components and versions in parallel using channels
func (s *Storage) saveKnowledgeBaseParallel(tx *sql.Tx, kb *types.KnowledgeBase) error {
	// Determine optimal number of workers based on available CPU cores and dataset size
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Cap at 8 to avoid overwhelming the database
	}
	if len(kb.Components) < numWorkers {
		numWorkers = len(kb.Components) // Don't create more workers than components
	}

	// Ensure we have at least one worker
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Create channels for parallel processing with appropriate buffer sizes
	componentChan := make(chan *types.Component, numWorkers*2)
	versionChan := make(chan *versionInsertTask, numWorkers*20) // Buffer for versions
	errorChan := make(chan error, numWorkers)                   // Buffer errors for all workers
	stopChan := make(chan struct{})                             // Channel to signal workers to stop

	// Start worker goroutines for processing components
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go s.componentWorker(tx, componentChan, versionChan, errorChan, stopChan, &wg)
	}

	// Start version processing workers
	var versionWg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		versionWg.Add(1)
		go s.versionWorker(tx, versionChan, errorChan, stopChan, &versionWg)
	}

	// Send components to workers
	go func() {
		defer close(componentChan)
		for _, component := range kb.Components {
			select {
			case componentChan <- &component:
			case <-stopChan:
				// Stop signal received, stop sending components
				return
			}
		}
	}()

	// Wait for all components to be processed
	go func() {
		wg.Wait()
		close(versionChan)
	}()

	// Wait for all versions to be processed
	go func() {
		versionWg.Wait()
	}()

	// Monitor for errors and completion
	select {
	case err := <-errorChan:
		// Signal all workers to stop
		close(stopChan)
		// Wait for workers to finish
		wg.Wait()
		versionWg.Wait()
		return fmt.Errorf("parallel processing error: %w", err)
	default:
		// No errors, continue with processing
	}

	// Wait for all processing to complete
	wg.Wait()
	versionWg.Wait()

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// versionInsertTask represents a task for inserting a version
type versionInsertTask struct {
	componentID int64
	version     *types.Version
}

// componentWorker processes components in parallel
func (s *Storage) componentWorker(tx *sql.Tx, componentChan <-chan *types.Component, versionChan chan<- *versionInsertTask, errorChan chan<- error, stopChan <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	for component := range componentChan {
		// Check if we should stop
		select {
		case <-stopChan:
			return
		default:
		}

		// Insert component and get its ID
		componentID, err := s.insertComponent(tx, component)
		if err != nil {
			// Log the specific error for debugging
			errMsg := fmt.Errorf("failed to insert component %s (language: %s): %w",
				component.Name, string(component.Language), err)

			select {
			case errorChan <- errMsg:
			default:
				// Error channel is full, try to send again
				errorChan <- errMsg
			}
			return
		}

		// Send versions to version workers for parallel processing
		// Use non-blocking sends to avoid blocking when version workers are busy
		for _, version := range component.Versions {
			select {
			case versionChan <- &versionInsertTask{
				componentID: componentID,
				version:     &version,
			}:
			case <-stopChan:
				// Stop signal received, stop processing
				return
			}
		}
	}
}

// versionWorker processes versions in parallel with batch processing
func (s *Storage) versionWorker(tx *sql.Tx, versionChan <-chan *versionInsertTask, errorChan chan<- error, stopChan <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	// Process versions in batches for better performance
	const batchSize = 50
	var batch []*versionInsertTask

	// Process versions in batches
	for task := range versionChan {
		// Check if we should stop
		select {
		case <-stopChan:
			return
		default:
		}

		batch = append(batch, task)

		// Process batch when it reaches the target size or channel is empty
		if len(batch) >= batchSize {
			if err := s.insertVersionsBatch(tx, batch); err != nil {
				select {
				case errorChan <- fmt.Errorf("failed to insert version batch: %w", err):
				default:
					// Error channel is full, try to send again
					errorChan <- fmt.Errorf("failed to insert version batch: %w", err)
				}
				return
			}
			batch = batch[:0] // Reset batch
		}
	}

	// Process remaining versions in the final batch
	if len(batch) > 0 {
		if err := s.insertVersionsBatch(tx, batch); err != nil {
			select {
			case errorChan <- fmt.Errorf("failed to insert final version batch: %w", err):
			default:
				// Error channel is full, try to send again
				errorChan <- fmt.Errorf("failed to insert final version batch: %w", err)
			}
		}
	}
}

// insertVersionsBatch inserts multiple versions in a single database operation
func (s *Storage) insertVersionsBatch(tx *sql.Tx, tasks []*versionInsertTask) error {
	if len(tasks) == 0 {
		return nil
	}

	// For now, fall back to individual inserts to maintain compatibility
	// In the future, this could be optimized with a proper batch INSERT statement
	for _, task := range tasks {
		if err := s.insertVersion(tx, task.componentID, task.version); err != nil {
			return fmt.Errorf("failed to insert version %s for component %d: %w", task.version.Name, task.componentID, err)
		}
	}
	return nil
}

// initDatabase creates the database schema and handles migrations
func initDatabase(db *sql.DB) error {
	// Check if we need to migrate from old schema
	if err := migrateDatabaseIfNeeded(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Create components table
	componentsTable := `
	CREATE TABLE IF NOT EXISTS components (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		category TEXT,
		status TEXT,
		support_level TEXT,
		language TEXT NOT NULL,
		description TEXT,
		repository TEXT NOT NULL,
		registry_url TEXT,
		homepage TEXT,
		tags TEXT, -- JSON array
		maintainers TEXT, -- JSON array
		license TEXT,
		last_updated DATETIME NOT NULL,
		instrumentation_targets TEXT, -- JSON array
		documentation_url TEXT,
		examples_url TEXT,
		migration_guide_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(name, language)
	)`

	// Create versions table
	versionsTable := `
	CREATE TABLE IF NOT EXISTS versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		component_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		release_date DATETIME NOT NULL,
		dependencies TEXT, -- JSON object
		min_runtime_version TEXT,
		max_runtime_version TEXT,
		status TEXT,
		deprecated BOOLEAN DEFAULT FALSE,
		breaking_changes TEXT, -- JSON array
		metadata TEXT, -- JSON object
		registry_url TEXT,
		npm_url TEXT,
		github_url TEXT,
		changelog_url TEXT,
		core_version TEXT,
		experimental_version TEXT,
		compatible TEXT, -- JSON array
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (component_id) REFERENCES components(id) ON DELETE CASCADE
	)`

	// Create indexes for better query performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_components_name_language ON components(name, language)",
		"CREATE INDEX IF NOT EXISTS idx_components_type ON components(type)",
		"CREATE INDEX IF NOT EXISTS idx_components_language ON components(language)",
		"CREATE INDEX IF NOT EXISTS idx_components_category ON components(category)",
		"CREATE INDEX IF NOT EXISTS idx_components_status ON components(status)",
		"CREATE INDEX IF NOT EXISTS idx_components_support_level ON components(support_level)",
		"CREATE INDEX IF NOT EXISTS idx_components_last_updated ON components(last_updated)",
		"CREATE INDEX IF NOT EXISTS idx_versions_component_id ON versions(component_id)",
		"CREATE INDEX IF NOT EXISTS idx_versions_name ON versions(name)",
		"CREATE INDEX IF NOT EXISTS idx_components_name ON components(name)",
		"CREATE INDEX IF NOT EXISTS idx_versions_status ON versions(status)",
	}

	// Execute schema creation
	if _, err := db.Exec(componentsTable); err != nil {
		return fmt.Errorf("failed to create components table: %w", err)
	}

	if _, err := db.Exec(versionsTable); err != nil {
		return fmt.Errorf("failed to create versions table: %w", err)
	}

	// Execute index creation
	for _, index := range indexes {
		if _, err := db.Exec(index); err != nil {
			return fmt.Errorf("failed to create index %s: %w", index, err)
		}
	}

	return nil
}

// migrateDatabaseIfNeeded handles database schema migrations
func migrateDatabaseIfNeeded(db *sql.DB) error {
	// Check if components table exists
	var tableExists bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM sqlite_master
		WHERE type='table' AND name='components'
	`).Scan(&tableExists)

	if err != nil {
		// Table doesn't exist, no migration needed
		return nil
	}

	if !tableExists {
		// Table doesn't exist, no migration needed
		return nil
	}

	// Check if the old UNIQUE constraint on name exists
	var constraintExists bool
	err = db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM sqlite_master
		WHERE type='index' AND name='idx_components_name'
	`).Scan(&constraintExists)

	if err != nil || !constraintExists {
		// No old constraint, no migration needed
		return nil
	}

	// We need to migrate from the old schema
	fmt.Println("Migrating database schema to support parallel processing...")

	// Start a transaction for the migration
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	// Create a temporary table with the new schema
	tempTable := `
	CREATE TABLE components_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		category TEXT,
		status TEXT,
		support_level TEXT,
		language TEXT NOT NULL,
		description TEXT,
		repository TEXT NOT NULL,
		registry_url TEXT,
		homepage TEXT,
		tags TEXT,
		maintainers TEXT,
		license TEXT,
		last_updated DATETIME NOT NULL,
		instrumentation_targets TEXT,
		documentation_url TEXT,
		examples_url TEXT,
		migration_guide_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(name, language)
	)`

	if _, err := tx.Exec(tempTable); err != nil {
		return fmt.Errorf("failed to create temporary table: %w", err)
	}

	// Copy data from old table to new table
	if _, err := tx.Exec(`
		INSERT INTO components_new
		SELECT * FROM components
	`); err != nil {
		return fmt.Errorf("failed to copy data to temporary table: %w", err)
	}

	// Drop old table
	if _, err := tx.Exec("DROP TABLE components"); err != nil {
		return fmt.Errorf("failed to drop old table: %w", err)
	}

	// Rename new table
	if _, err := tx.Exec("ALTER TABLE components_new RENAME TO components"); err != nil {
		return fmt.Errorf("failed to rename new table: %w", err)
	}

	// Create new indexes
	indexes := []string{
		"CREATE INDEX idx_components_name_language ON components(name, language)",
		"CREATE INDEX idx_components_type ON components(type)",
		"CREATE INDEX idx_components_language ON components(language)",
		"CREATE INDEX idx_components_category ON components(category)",
		"CREATE INDEX idx_components_status ON components(status)",
		"CREATE INDEX idx_components_support_level ON components(support_level)",
		"CREATE INDEX idx_components_last_updated ON components(last_updated)",
		"CREATE INDEX idx_components_name ON components(name)",
	}

	for _, index := range indexes {
		if _, err := tx.Exec(index); err != nil {
			return fmt.Errorf("failed to create index %s: %w", index, err)
		}
	}

	// Commit the migration
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	fmt.Println("Database migration completed successfully!")
	return nil
}

// insertComponent inserts a component into the database and returns its ID
// Uses UPSERT (INSERT OR REPLACE) to handle potential duplicates gracefully
func (s *Storage) insertComponent(tx *sql.Tx, component *types.Component) (int64, error) {
	// Convert slices to JSON strings
	tagsJSON, _ := json.Marshal(component.Tags)
	maintainersJSON, _ := json.Marshal(component.Maintainers)
	targetsJSON, _ := json.Marshal(component.InstrumentationTargets)

	// Use INSERT OR REPLACE to handle duplicates gracefully
	query := `
		INSERT OR REPLACE INTO components (
			name, type, category, status, support_level, language, description,
			repository, registry_url, homepage, tags, maintainers, license,
			last_updated, instrumentation_targets, documentation_url,
			examples_url, migration_guide_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := tx.Exec(query,
		component.Name, string(component.Type), string(component.Category),
		string(component.Status), string(component.SupportLevel), string(component.Language),
		component.Description, component.Repository, component.RegistryURL,
		component.Homepage, string(tagsJSON), string(maintainersJSON),
		component.License, component.LastUpdated, string(targetsJSON),
		component.DocumentationURL, component.ExamplesURL, component.MigrationGuideURL,
	)
	if err != nil {
		return 0, err
	}

	// For INSERT OR REPLACE, we need to get the ID differently
	// First try to get the last insert ID
	lastID, err := result.LastInsertId()
	if err == nil && lastID > 0 {
		return lastID, nil
	}

	// If LastInsertId() fails (which can happen with REPLACE),
	// query for the component ID
	var componentID int64
	err = tx.QueryRow(`
		SELECT id FROM components
		WHERE name = ? AND language = ?
	`, component.Name, string(component.Language)).Scan(&componentID)

	if err != nil {
		return 0, fmt.Errorf("failed to get component ID after insert/replace: %w", err)
	}

	return componentID, nil
}

// insertVersion inserts a version into the database
func (s *Storage) insertVersion(tx *sql.Tx, componentID int64, version *types.Version) error {
	// Convert maps and slices to JSON strings
	depsJSON, _ := json.Marshal(version.Dependencies)
	breakingJSON, _ := json.Marshal(version.BreakingChanges)
	metadataJSON, _ := json.Marshal(version.Metadata)
	compatibleJSON, _ := json.Marshal(version.Compatible)

	query := `
		INSERT INTO versions (
			component_id, name, release_date, dependencies, min_runtime_version,
			max_runtime_version, status, deprecated, breaking_changes, metadata,
			registry_url, npm_url, github_url, changelog_url, core_version,
			experimental_version, compatible
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := tx.Exec(query,
		componentID, version.Name, version.ReleaseDate, string(depsJSON),
		version.MinRuntimeVersion, version.MaxRuntimeVersion, string(version.Status),
		version.Deprecated, string(breakingJSON), string(metadataJSON),
		version.RegistryURL, version.NPMURL, version.GitHubURL, version.ChangelogURL,
		version.CoreVersion, version.ExperimentalVersion, string(compatibleJSON),
	)

	return err
}

// loadComponents loads all components from the database
func (s *Storage) loadComponents() ([]types.Component, error) {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		ORDER BY name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var components []types.Component
	for rows.Next() {
		var component types.Component
		var id int64
		var tagsJSON, maintainersJSON, targetsJSON string

		err := rows.Scan(
			&id, &component.Name, &component.Type, &component.Category,
			&component.Status, &component.SupportLevel, &component.Language,
			&component.Description, &component.Repository, &component.RegistryURL,
			&component.Homepage, &tagsJSON, &maintainersJSON, &component.License,
			&component.LastUpdated, &targetsJSON, &component.DocumentationURL,
			&component.ExamplesURL, &component.MigrationGuideURL,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		json.Unmarshal([]byte(tagsJSON), &component.Tags)
		json.Unmarshal([]byte(maintainersJSON), &component.Maintainers)
		json.Unmarshal([]byte(targetsJSON), &component.InstrumentationTargets)

		// Load versions for this component
		versions, err := s.loadVersions(id)
		if err != nil {
			return nil, err
		}
		component.Versions = versions

		components = append(components, component)
	}

	return components, nil
}

// loadVersions loads all versions for a component
func (s *Storage) loadVersions(componentID int64) ([]types.Version, error) {
	query := `
		SELECT name, release_date, dependencies, min_runtime_version, max_runtime_version,
		       status, deprecated, breaking_changes, metadata, registry_url, npm_url,
		       github_url, changelog_url, core_version, experimental_version, compatible
		FROM versions
		WHERE component_id = ?
		ORDER BY release_date DESC
	`

	rows, err := s.db.Query(query, componentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []types.Version
	for rows.Next() {
		var version types.Version
		var depsJSON, breakingJSON, metadataJSON, compatibleJSON string

		err := rows.Scan(
			&version.Name, &version.ReleaseDate, &depsJSON, &version.MinRuntimeVersion,
			&version.MaxRuntimeVersion, &version.Status, &version.Deprecated,
			&breakingJSON, &metadataJSON, &version.RegistryURL, &version.NPMURL,
			&version.GitHubURL, &version.ChangelogURL, &version.CoreVersion,
			&version.ExperimentalVersion, &compatibleJSON,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		json.Unmarshal([]byte(depsJSON), &version.Dependencies)
		json.Unmarshal([]byte(breakingJSON), &version.BreakingChanges)
		json.Unmarshal([]byte(metadataJSON), &version.Metadata)
		json.Unmarshal([]byte(compatibleJSON), &version.Compatible)

		versions = append(versions, version)
	}

	return versions, nil
}

// calculateStatistics calculates statistics for the knowledge base using SQL aggregation
func (s *Storage) calculateStatistics() (types.Statistics, error) {
	stats := types.Statistics{
		ByLanguage:     make(map[string]int),
		ByType:         make(map[string]int),
		ByCategory:     make(map[string]int),
		ByStatus:       make(map[string]int),
		BySupportLevel: make(map[string]int),
		LastUpdate:     time.Now(),
		Source:         "SQLite Database",
	}

	// Get total component count
	err := s.db.QueryRow("SELECT COUNT(*) FROM components").Scan(&stats.TotalComponents)
	if err != nil {
		return stats, fmt.Errorf("failed to get total components count: %w", err)
	}

	// Get total version count
	err = s.db.QueryRow("SELECT COUNT(*) FROM versions").Scan(&stats.TotalVersions)
	if err != nil {
		return stats, fmt.Errorf("failed to get total versions count: %w", err)
	}

	// Count by language
	rows, err := s.db.Query("SELECT language, COUNT(*) FROM components GROUP BY language")
	if err != nil {
		return stats, fmt.Errorf("failed to get language statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var language string
		var count int
		if err := rows.Scan(&language, &count); err == nil {
			stats.ByLanguage[language] = count
		}
	}

	// Count by type
	rows, err = s.db.Query("SELECT type, COUNT(*) FROM components GROUP BY type")
	if err != nil {
		return stats, fmt.Errorf("failed to get type statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var componentType string
		var count int
		if err := rows.Scan(&componentType, &count); err == nil {
			stats.ByType[componentType] = count
		}
	}

	// Count by category (excluding empty categories)
	rows, err = s.db.Query("SELECT category, COUNT(*) FROM components WHERE category != '' GROUP BY category")
	if err != nil {
		return stats, fmt.Errorf("failed to get category statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err == nil {
			stats.ByCategory[category] = count
		}
	}

	// Count by status (excluding empty statuses)
	rows, err = s.db.Query("SELECT status, COUNT(*) FROM components WHERE status != '' GROUP BY status")
	if err != nil {
		return stats, fmt.Errorf("failed to get status statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err == nil {
			stats.ByStatus[status] = count
		}
	}

	// Count by support level (excluding empty support levels)
	rows, err = s.db.Query("SELECT support_level, COUNT(*) FROM components WHERE support_level != '' GROUP BY support_level")
	if err != nil {
		return stats, fmt.Errorf("failed to get support level statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var supportLevel string
		var count int
		if err := rows.Scan(&supportLevel, &count); err == nil {
			stats.BySupportLevel[supportLevel] = count
		}
	}

	return stats, nil
}

// buildQuerySQL builds a SQL query based on the query criteria
func (s *Storage) buildQuerySQL(query Query) (string, []interface{}) {
	baseQuery := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE 1=1
	`

	var args []interface{}
	argIndex := 1

	// Add filters based on query criteria
	if query.Language != "" {
		baseQuery += " AND language = ?"
		args = append(args, query.Language)
		argIndex++
	}

	if query.Type != "" {
		baseQuery += " AND type = ?"
		args = append(args, query.Type)
		argIndex++
	}

	if query.Category != "" {
		baseQuery += " AND category = ?"
		args = append(args, query.Category)
		argIndex++
	}

	if query.Status != "" {
		baseQuery += " AND status = ?"
		args = append(args, query.Status)
		argIndex++
	}

	if query.SupportLevel != "" {
		baseQuery += " AND support_level = ?"
		args = append(args, query.SupportLevel)
		argIndex++
	}

	if query.Name != "" {
		baseQuery += " AND name LIKE ?"
		args = append(args, "%"+query.Name+"%")
		argIndex++
	}

	if query.Framework != "" {
		baseQuery += " AND instrumentation_targets LIKE ?"
		args = append(args, "%"+query.Framework+"%")
		argIndex++
	}

	if !query.MinDate.IsZero() {
		baseQuery += " AND last_updated >= ?"
		args = append(args, query.MinDate)
		argIndex++
	}

	if !query.MaxDate.IsZero() {
		baseQuery += " AND last_updated <= ?"
		args = append(args, query.MaxDate)
		argIndex++
	}

	// Add ORDER BY
	baseQuery += " ORDER BY name"

	// Add pagination if specified
	if query.Limit > 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
		if query.Offset > 0 {
			baseQuery += fmt.Sprintf(" OFFSET %d", query.Offset)
		}
	}

	return baseQuery, args
}

// getQueryTotalCount gets the total count of components matching the query criteria (without pagination)
func (s *Storage) getQueryTotalCount(query Query) int {
	baseQuery := "SELECT COUNT(*) FROM components WHERE 1=1"
	var args []interface{}

	// Add the same filters as buildQuerySQL but only for counting
	if query.Language != "" {
		baseQuery += " AND language = ?"
		args = append(args, query.Language)
	}

	if query.Type != "" {
		baseQuery += " AND type = ?"
		args = append(args, query.Type)
	}

	if query.Category != "" {
		baseQuery += " AND category = ?"
		args = append(args, query.Category)
	}

	if query.Status != "" {
		baseQuery += " AND status = ?"
		args = append(args, query.Status)
	}

	if query.SupportLevel != "" {
		baseQuery += " AND support_level = ?"
		args = append(args, query.SupportLevel)
	}

	if query.Name != "" {
		baseQuery += " AND name LIKE ?"
		args = append(args, "%"+query.Name+"%")
	}

	if query.Framework != "" {
		baseQuery += " AND instrumentation_targets LIKE ?"
		args = append(args, "%"+query.Framework+"%")
	}

	if !query.MinDate.IsZero() {
		baseQuery += " AND last_updated >= ?"
		args = append(args, query.MinDate)
	}

	if !query.MaxDate.IsZero() {
		baseQuery += " AND last_updated <= ?"
		args = append(args, query.MaxDate)
	}

	var count int
	err := s.db.QueryRow(baseQuery, args...).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// scanComponents scans database rows into Component structs with optional lazy loading of versions
func (s *Storage) scanComponents(rows *sql.Rows) []types.Component {
	return s.scanComponentsWithVersions(rows, true)
}

// scanComponentsWithVersions scans database rows into Component structs with control over version loading
func (s *Storage) scanComponentsWithVersions(rows *sql.Rows, loadVersions bool) []types.Component {
	var components []types.Component
	for rows.Next() {
		var component types.Component
		var id int64
		var tagsJSON, maintainersJSON, targetsJSON string

		err := rows.Scan(
			&id, &component.Name, &component.Type, &component.Category,
			&component.Status, &component.SupportLevel, &component.Language,
			&component.Description, &component.Repository, &component.RegistryURL,
			&component.Homepage, &tagsJSON, &maintainersJSON, &component.License,
			&component.LastUpdated, &targetsJSON, &component.DocumentationURL,
			&component.ExamplesURL, &component.MigrationGuideURL,
		)
		if err != nil {
			continue // Skip this row on error
		}

		// Parse JSON fields
		json.Unmarshal([]byte(tagsJSON), &component.Tags)
		json.Unmarshal([]byte(maintainersJSON), &component.Maintainers)
		json.Unmarshal([]byte(targetsJSON), &component.InstrumentationTargets)

		// Load versions only if requested (lazy loading support)
		if loadVersions {
			versions, err := s.loadVersions(id)
			if err == nil {
				component.Versions = versions
			}
		} else {
			// Initialize empty versions slice for lazy loading
			component.Versions = []types.Version{}
		}

		components = append(components, component)
	}

	return components
}

// LoadKnowledgeBase loads the knowledge base metadata from the SQLite database without loading all components
// Components are loaded on-demand through query methods
func (s *Storage) LoadKnowledgeBase(filename string) (*types.KnowledgeBase, error) {
	// Calculate statistics using SQL aggregation instead of loading all components
	stats, err := s.calculateStatistics()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate statistics: %w", err)
	}

	// Create knowledge base with empty components slice - components loaded on-demand
	kb := &types.KnowledgeBase{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Components:    []types.Component{}, // Empty - use query methods to load specific components
		Statistics:    stats,
	}

	return kb, nil
}

// Query represents a query against the knowledge base
type Query struct {
	Language     string
	Type         string
	Category     string
	Status       string
	SupportLevel string
	Name         string
	Version      string
	MinDate      time.Time
	MaxDate      time.Time
	Tags         []string
	Maintainers  []string
	Framework    string // For instrumentation targets
	// Pagination support
	Limit  int // Maximum number of results to return (0 = no limit)
	Offset int // Number of results to skip
}

// QueryResult represents the result of a query
type QueryResult struct {
	Components []types.Component
	Total      int // Total number of matching components (ignoring pagination)
	Returned   int // Number of components returned in this result
	Query      Query
	HasMore    bool // Whether there are more results available
}

// QueryKnowledgeBase queries the knowledge base based on criteria using SQLite with pagination support
func (s *Storage) QueryKnowledgeBase(kb *types.KnowledgeBase, query Query) *QueryResult {
	// First, get the total count without pagination
	totalCount := s.getQueryTotalCount(query)

	// Build the SQL query with pagination
	sqlQuery, args := s.buildQuerySQL(query)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		// Return empty result on error
		return &QueryResult{
			Components: []types.Component{},
			Total:      0,
			Returned:   0,
			Query:      query,
			HasMore:    false,
		}
	}
	defer rows.Close()

	var results []types.Component
	for rows.Next() {
		var component types.Component
		var id int64
		var tagsJSON, maintainersJSON, targetsJSON string

		err := rows.Scan(
			&id, &component.Name, &component.Type, &component.Category,
			&component.Status, &component.SupportLevel, &component.Language,
			&component.Description, &component.Repository, &component.RegistryURL,
			&component.Homepage, &tagsJSON, &maintainersJSON, &component.License,
			&component.LastUpdated, &targetsJSON, &component.DocumentationURL,
			&component.ExamplesURL, &component.MigrationGuideURL,
		)
		if err != nil {
			continue // Skip this row on error
		}

		// Parse JSON fields
		json.Unmarshal([]byte(tagsJSON), &component.Tags)
		json.Unmarshal([]byte(maintainersJSON), &component.Maintainers)
		json.Unmarshal([]byte(targetsJSON), &component.InstrumentationTargets)

		// Load versions for this component
		versions, err := s.loadVersions(id)
		if err == nil {
			component.Versions = versions
		}

		results = append(results, component)
	}

	// Calculate if there are more results
	hasMore := false
	if query.Limit > 0 && len(results) == query.Limit {
		hasMore = (query.Offset + len(results)) < totalCount
	}

	return &QueryResult{
		Components: results,
		Total:      totalCount,
		Returned:   len(results),
		Query:      query,
		HasMore:    hasMore,
	}
}

// GetComponentsByType returns all components of a specific type
func (s *Storage) GetComponentsByType(kb *types.KnowledgeBase, componentType types.ComponentType) []types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE type = ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, string(componentType))
	if err != nil {
		return []types.Component{}
	}
	defer rows.Close()

	return s.scanComponents(rows)
}

// GetComponentsByLanguage returns all components for a specific language
func (s *Storage) GetComponentsByLanguage(kb *types.KnowledgeBase, language types.ComponentLanguage) []types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE language = ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, string(language))
	if err != nil {
		return []types.Component{}
	}
	defer rows.Close()

	return s.scanComponents(rows)
}

// GetLatestVersions returns the latest version of each component
func (s *Storage) GetLatestVersions(kb *types.KnowledgeBase) map[string]types.Version {
	query := `
		SELECT c.name, v.name, v.release_date, v.dependencies, v.min_runtime_version,
		       v.max_runtime_version, v.status, v.deprecated, v.breaking_changes, v.metadata,
		       v.registry_url, v.npm_url, v.github_url, v.changelog_url, v.core_version,
		       v.experimental_version, v.compatible
		FROM components c
		INNER JOIN (
			SELECT component_id, MAX(release_date) as max_date
			FROM versions
			GROUP BY component_id
		) latest ON c.id = latest.component_id
		INNER JOIN versions v ON v.component_id = c.id AND v.release_date = latest.max_date
		ORDER BY c.name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return make(map[string]types.Version)
	}
	defer rows.Close()

	latestVersions := make(map[string]types.Version)
	for rows.Next() {
		var version types.Version
		var componentName string
		var depsJSON, breakingJSON, metadataJSON, compatibleJSON string

		err := rows.Scan(
			&componentName, &version.Name, &version.ReleaseDate, &depsJSON,
			&version.MinRuntimeVersion, &version.MaxRuntimeVersion, &version.Status,
			&version.Deprecated, &breakingJSON, &metadataJSON, &version.RegistryURL,
			&version.NPMURL, &version.GitHubURL, &version.ChangelogURL,
			&version.CoreVersion, &version.ExperimentalVersion, &compatibleJSON,
		)
		if err != nil {
			continue
		}

		// Parse JSON fields
		json.Unmarshal([]byte(depsJSON), &version.Dependencies)
		json.Unmarshal([]byte(breakingJSON), &version.BreakingChanges)
		json.Unmarshal([]byte(metadataJSON), &version.Metadata)
		json.Unmarshal([]byte(compatibleJSON), &version.Compatible)

		latestVersions[componentName] = version
	}

	return latestVersions
}

// GetComponentByName returns a component by name
func (s *Storage) GetComponentByName(kb *types.KnowledgeBase, name string) *types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE name = ?
		LIMIT 1
	`

	row := s.db.QueryRow(query, name)
	var component types.Component
	var id int64
	var tagsJSON, maintainersJSON, targetsJSON string

	err := row.Scan(
		&id, &component.Name, &component.Type, &component.Category,
		&component.Status, &component.SupportLevel, &component.Language,
		&component.Description, &component.Repository, &component.RegistryURL,
		&component.Homepage, &tagsJSON, &maintainersJSON, &component.License,
		&component.LastUpdated, &targetsJSON, &component.DocumentationURL,
		&component.ExamplesURL, &component.MigrationGuideURL,
	)
	if err != nil {
		return nil
	}

	// Parse JSON fields
	json.Unmarshal([]byte(tagsJSON), &component.Tags)
	json.Unmarshal([]byte(maintainersJSON), &component.Maintainers)
	json.Unmarshal([]byte(targetsJSON), &component.InstrumentationTargets)

	// Load versions for this component
	versions, err := s.loadVersions(id)
	if err == nil {
		component.Versions = versions
	}

	return &component
}

// GetComponentsByCategory returns all components of a specific category
func (s *Storage) GetComponentsByCategory(kb *types.KnowledgeBase, category types.ComponentCategory) []types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE category = ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, string(category))
	if err != nil {
		return []types.Component{}
	}
	defer rows.Close()

	return s.scanComponents(rows)
}

// GetComponentsByStatus returns all components with a specific status
func (s *Storage) GetComponentsByStatus(kb *types.KnowledgeBase, status types.ComponentStatus) []types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE status = ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, string(status))
	if err != nil {
		return []types.Component{}
	}
	defer rows.Close()

	return s.scanComponents(rows)
}

// GetComponentsBySupportLevel returns all components with a specific support level
func (s *Storage) GetComponentsBySupportLevel(kb *types.KnowledgeBase, supportLevel types.SupportLevel) []types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE support_level = ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, string(supportLevel))
	if err != nil {
		return []types.Component{}
	}
	defer rows.Close()

	return s.scanComponents(rows)
}

// GetInstrumentationsByFramework returns all instrumentations for a specific framework
func (s *Storage) GetInstrumentationsByFramework(kb *types.KnowledgeBase, framework string) []types.Component {
	query := `
		SELECT id, name, type, category, status, support_level, language, description,
		       repository, registry_url, homepage, tags, maintainers, license,
		       last_updated, instrumentation_targets, documentation_url,
		       examples_url, migration_guide_url
		FROM components
		WHERE type = ? AND instrumentation_targets LIKE ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, string(types.ComponentTypeInstrumentation), "%"+framework+"%")
	if err != nil {
		return []types.Component{}
	}
	defer rows.Close()

	return s.scanComponents(rows)
}

// GetCompatibleVersions returns compatible versions for a given component and version
func (s *Storage) GetCompatibleVersions(kb *types.KnowledgeBase, componentName, version string) []types.CompatibleComponent {
	query := `
		SELECT v.compatible
		FROM components c
		INNER JOIN versions v ON c.id = v.component_id
		WHERE c.name = ? AND v.name = ?
		LIMIT 1
	`

	row := s.db.QueryRow(query, componentName, version)
	var compatibleJSON string

	err := row.Scan(&compatibleJSON)
	if err != nil {
		return nil
	}

	var compatible []types.CompatibleComponent
	json.Unmarshal([]byte(compatibleJSON), &compatible)
	return compatible
}

// GetBreakingChanges returns breaking changes for a given component
func (s *Storage) GetBreakingChanges(kb *types.KnowledgeBase, componentName string) []types.BreakingChange {
	query := `
		SELECT v.breaking_changes
		FROM components c
		INNER JOIN versions v ON c.id = v.component_id
		WHERE c.name = ?
	`

	rows, err := s.db.Query(query, componentName)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var allBreakingChanges []types.BreakingChange
	for rows.Next() {
		var breakingJSON string
		err := rows.Scan(&breakingJSON)
		if err != nil {
			continue
		}

		var breakingChanges []types.BreakingChange
		json.Unmarshal([]byte(breakingJSON), &breakingChanges)
		allBreakingChanges = append(allBreakingChanges, breakingChanges...)
	}

	return allBreakingChanges
}

// GetComponentsLight returns components without loading their versions (for performance)
func (s *Storage) GetComponentsLight(query Query) *QueryResult {
	// First, get the total count without pagination
	totalCount := s.getQueryTotalCount(query)

	// Build the SQL query with pagination
	sqlQuery, args := s.buildQuerySQL(query)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return &QueryResult{
			Components: []types.Component{},
			Total:      0,
			Returned:   0,
			Query:      query,
			HasMore:    false,
		}
	}
	defer rows.Close()

	// Scan components without loading versions for better performance
	results := s.scanComponentsWithVersions(rows, false)

	// Calculate if there are more results
	hasMore := false
	if query.Limit > 0 && len(results) == query.Limit {
		hasMore = (query.Offset + len(results)) < totalCount
	}

	return &QueryResult{
		Components: results,
		Total:      totalCount,
		Returned:   len(results),
		Query:      query,
		HasMore:    hasMore,
	}
}

// LoadComponentVersions loads versions for a specific component by name
func (s *Storage) LoadComponentVersions(componentName string) ([]types.Version, error) {
	// First get the component ID
	var componentID int64
	err := s.db.QueryRow("SELECT id FROM components WHERE name = ?", componentName).Scan(&componentID)
	if err != nil {
		return nil, fmt.Errorf("component %s not found: %w", componentName, err)
	}

	// Load versions for this component
	return s.loadVersions(componentID)
}

// GetComponentCount returns the total number of components in the database
func (s *Storage) GetComponentCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM components").Scan(&count)
	return count, err
}

// GetVersionCount returns the total number of versions in the database
func (s *Storage) GetVersionCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM versions").Scan(&count)
	return count, err
}
