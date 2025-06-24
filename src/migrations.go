package src

import (
	"fmt"
	"log"
)

type Migration struct {
	ID          int
	Description string
	SQL         string
}

var migrations = []Migration{
	{
		ID:          1,
		Description: "Add cracked_password column",
		SQL: `
			ALTER TABLE scanned ADD COLUMN cracked_password TEXT;
		`,
	},
	{
		ID:          2,
		Description: "Rename scanned table to aps",
		SQL: `
			ALTER TABLE scanned RENAME TO aps;
		`,
	},
	{
		ID:          3,
		Description: "Create probes table",
		SQL: `
			CREATE TABLE IF NOT EXISTS probes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				essid TEXT,
				mac TEXT,
				signal INTEGER,
				channel TEXT,
				vendor TEXT,
				probed_at DATETIME,
				UNIQUE(essid, mac)
			);
		`,
	},
}

func (d *Database) RunMigrations() error {
	// Create migrations table if it doesn't exist
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY,
			description TEXT,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	// Check and apply each migration
	for _, migration := range migrations {
		var count int
		err := d.db.QueryRow("SELECT COUNT(*) FROM migrations WHERE id = ?", migration.ID).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration %d: %v", migration.ID, err)
		}

		if count > 0 {
			// Migration already applied
			continue
		}

		// Apply migration
		log.Printf("[MIGRATION] Applying migration %d: %s", migration.ID, migration.Description)

		// Start transaction
		tx, err := d.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction for migration %d: %v", migration.ID, err)
		}

		// Execute migration SQL
		_, err = tx.Exec(migration.SQL)
		if err != nil {
			tx.Rollback()
			// Check if error is because column already exists (for safety)
			if isColumnExistsError(err) {
				log.Printf("[MIGRATION] Migration %d: columns already exist, marking as applied", migration.ID)
				// Mark migration as applied anyway
				_, err = d.db.Exec("INSERT INTO migrations (id, description) VALUES (?, ?)",
					migration.ID, migration.Description)
				if err != nil {
					return fmt.Errorf("failed to mark migration %d as applied: %v", migration.ID, err)
				}
				continue
			}
			return fmt.Errorf("failed to apply migration %d: %v", migration.ID, err)
		}

		// Record migration as applied
		_, err = tx.Exec("INSERT INTO migrations (id, description) VALUES (?, ?)",
			migration.ID, migration.Description)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %v", migration.ID, err)
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %v", migration.ID, err)
		}

		log.Printf("[MIGRATION] Successfully applied migration %d", migration.ID)
	}

	return nil
}

func isColumnExistsError(err error) bool {
	// SQLite error for duplicate column
	errStr := err.Error()
	return contains(errStr, "duplicate column name") ||
		contains(errStr, "already exists")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
