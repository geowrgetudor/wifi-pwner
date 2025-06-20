package src

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Cleaner struct {
	workingDir string
}

func NewCleaner(workingDir string) *Cleaner {
	return &Cleaner{workingDir: workingDir}
}

func (c *Cleaner) Clean() error {
	// Remove scanned directory
	scannedDir := filepath.Join(c.workingDir, "scanned")
	if err := os.RemoveAll(scannedDir); err != nil {
		return fmt.Errorf("failed to remove scanned directory: %v", err)
	}

	// Remove database
	dbPath := filepath.Join(c.workingDir, "scanned.db")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove database: %v", err)
	}

	log.Println("[CLEAN] Database and captures cleared")
	return nil
}