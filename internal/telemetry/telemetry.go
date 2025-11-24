// Package telemetry provides foundation for learning system (Orca v2 philosophy).
// Phase 1: Basic telemetry infrastructure to enable future learning features.
package telemetry

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Telemetry tracks usage patterns for learning system capabilities.
type Telemetry struct {
	db *sql.DB
}

// Event represents a telemetry event.
type Event struct {
	Timestamp    time.Time
	Command      string
	PatternType  string
	PatternMatch bool
	DensityMode  string
	Theme        string
	Duration     time.Duration
	ExitCode     int
}

// NewTelemetry creates a new telemetry instance with local SQLite database.
// Database is stored in user's config directory for privacy.
func NewTelemetry(enabled bool) (*Telemetry, error) {
	if !enabled {
		return &Telemetry{db: nil}, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user config dir: %w", err)
	}

	dbPath := filepath.Join(configDir, "fo", "telemetry.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create telemetry directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open telemetry database: %w", err)
	}

	t := &Telemetry{db: db}
	if err := t.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return t, nil
}

// initSchema creates the telemetry database schema.
func (t *Telemetry) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS pattern_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		command TEXT NOT NULL,
		pattern_type TEXT,
		pattern_match INTEGER NOT NULL,
		density_mode TEXT,
		theme TEXT,
		duration_ms INTEGER,
		exit_code INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_pattern_events_timestamp ON pattern_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_pattern_events_command ON pattern_events(command);
	CREATE INDEX IF NOT EXISTS idx_pattern_events_pattern_type ON pattern_events(pattern_type);
	`

	_, err := t.db.Exec(schema)
	return err
}

// RecordEvent records a telemetry event.
func (t *Telemetry) RecordEvent(event Event) error {
	if t.db == nil {
		return nil // Telemetry disabled
	}

	query := `
		INSERT INTO pattern_events 
		(timestamp, command, pattern_type, pattern_match, density_mode, theme, duration_ms, exit_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := t.db.Exec(
		query,
		event.Timestamp.Format(time.RFC3339),
		event.Command,
		event.PatternType,
		event.PatternMatch,
		event.DensityMode,
		event.Theme,
		event.Duration.Milliseconds(),
		event.ExitCode,
	)

	return err
}

// GetPatternMatchRate returns the pattern match rate for a given command.
func (t *Telemetry) GetPatternMatchRate(command string) (float64, error) {
	if t.db == nil {
		return 0, nil
	}

	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN pattern_match = 1 THEN 1 ELSE 0 END) as matches
		FROM pattern_events
		WHERE command = ?
	`

	var total, matches int
	err := t.db.QueryRow(query, command).Scan(&total, &matches)
	if err != nil {
		return 0, err
	}

	if total == 0 {
		return 0, nil
	}

	return float64(matches) / float64(total), nil
}

// GetPatternUsage returns usage statistics for pattern types.
func (t *Telemetry) GetPatternUsage() (map[string]int, error) {
	if t.db == nil {
		return nil, nil
	}

	query := `
		SELECT pattern_type, COUNT(*) as count
		FROM pattern_events
		WHERE pattern_type IS NOT NULL
		GROUP BY pattern_type
	`

	rows, err := t.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usage := make(map[string]int)
	for rows.Next() {
		var patternType string
		var count int
		if err := rows.Scan(&patternType, &count); err != nil {
			return nil, err
		}
		usage[patternType] = count
	}

	return usage, rows.Err()
}

// Close closes the telemetry database connection.
func (t *Telemetry) Close() error {
	if t.db == nil {
		return nil
	}
	return t.db.Close()
}
