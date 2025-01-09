package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// initDB initializes the database connection and schema.
func initDB() {
	var err error
	db, err = sql.Open(config.Database.Driver, config.Database.Datasource)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS identifiers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identifier TEXT NOT NULL UNIQUE,
		locked_by TEXT,
		last_seen TIMESTAMP
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	log.Println("Database initialized and schema verified.")
}

// preloadIdentifiers preloads a set of identifiers into the database.
func preloadIdentifiers() {
	expandedIdentifiers := ExpandIdentifiers(config.Identifiers.Patterns)

	for _, id := range expandedIdentifiers {
		_, err := db.Exec("INSERT OR IGNORE INTO identifiers (identifier) VALUES (?)", id)
		if err != nil {
			log.Printf("Failed to preload identifier %s: %v", id, err)
		}
	}
	log.Printf("Preloaded %d identifiers into the database", len(expandedIdentifiers))
}

// releaseStaleIdentifiers clears stale identifier locks based on the stale timeout from the config.
func releaseStaleIdentifiers() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		threshold := time.Now().Add(-config.Server.StaleTimeout)
		result, err := db.Exec(`
			UPDATE identifiers
			SET locked_by = NULL, last_seen = NULL
			WHERE last_seen < ? AND locked_by IS NOT NULL`,
			threshold,
		)

		if err != nil {
			log.Printf("Error releasing stale identifiers: %v", err)
		} else {
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected > 0 {
				log.Printf("Expired %d stale client(s) due to timeout (%s)", rowsAffected, config.Server.StaleTimeout)
			}
		}
	}
}
