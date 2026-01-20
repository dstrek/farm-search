package db

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// DB wraps sqlx.DB with application-specific methods
type DB struct {
	*sqlx.DB
}

// New creates a new database connection and runs migrations
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sqlx.Connect("sqlite", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{db}, nil
}

func migrate(db *sqlx.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	_, err = db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Run additional migrations for existing databases
	runMigrations(db)

	return nil
}

// runMigrations handles schema changes for existing databases
func runMigrations(db *sqlx.DB) {
	// Add drive_time_sydney column if it doesn't exist
	db.Exec("ALTER TABLE properties ADD COLUMN drive_time_sydney INTEGER")
	// Add nearest town columns
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_town_1 TEXT")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_town_1_km REAL")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_town_2 TEXT")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_town_2_km REAL")
	// Add nearest town drive time columns
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_town_1_mins INTEGER")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_town_2_mins INTEGER")
	// Add nearest school columns
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_school_1 TEXT")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_school_1_km REAL")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_school_1_mins INTEGER")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_school_2 TEXT")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_school_2_km REAL")
	db.Exec("ALTER TABLE properties ADD COLUMN nearest_school_2_mins INTEGER")
}
