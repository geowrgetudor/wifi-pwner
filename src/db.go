package src

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(workingDir string) (*Database, error) {
	dbPath := filepath.Join(workingDir, "scanned.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create the main table with all columns in their final form
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS scanned (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bssid TEXT UNIQUE,
			essid TEXT,
			signal INTEGER,
			channel TEXT,
			encryption TEXT,
			handshake_path TEXT,
			status TEXT,
			last_scan DATETIME
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	database := &Database{db: db}

	// Run migrations for existing databases
	if err := database.RunMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	// Reset any targets that were left in "Scanning" status
	if err := database.ResetScanningStatus(); err != nil {
		// Log error but don't fail - this is not critical
		fmt.Printf("Warning: Failed to reset scanning status: %v\n", err)
	}

	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveTarget(target *Target, handshakePath string, status Status) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO scanned 
		(bssid, essid, signal, channel, encryption, handshake_path, status, last_scan) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		target.BSSID,
		target.ESSID,
		target.Signal,
		target.Channel,
		target.Encryption,
		handshakePath,
		string(status),
		time.Now(),
	)
	return err
}

func (d *Database) UpdateTargetPassword(bssid string, password string, status Status) error {
	_, err := d.db.Exec(`
		UPDATE scanned 
		SET cracked_password = ?, status = ?
		WHERE bssid = ?`,
		password,
		string(status),
		bssid,
	)
	return err
}

func (d *Database) GetTargetsForCracking() ([]map[string]interface{}, error) {
	query := `
		SELECT bssid, essid, handshake_path 
		FROM scanned 
		WHERE (status = ? OR status = ?) AND handshake_path != ''
	`

	rows, err := d.db.Query(query, string(StatusHandshakeCaptured), string(StatusFailedToCrack))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []map[string]interface{}
	for rows.Next() {
		var bssid, essid, handshakePath string
		err := rows.Scan(&bssid, &essid, &handshakePath)
		if err != nil {
			continue
		}

		target := map[string]interface{}{
			"bssid":         bssid,
			"essid":         essid,
			"handshakePath": handshakePath,
		}
		targets = append(targets, target)
	}

	return targets, nil
}

func (d *Database) ShouldSkipTarget(bssid string) (bool, error) {
	var status sql.NullString
	var lastScan sql.NullTime

	err := d.db.QueryRow(
		"SELECT status, last_scan FROM scanned WHERE bssid = ?",
		bssid,
	).Scan(&status, &lastScan)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !status.Valid {
		return false, nil
	}

	if status.String == string(StatusHandshakeCaptured) || status.String == string(StatusCracked) || status.String == string(StatusFailedToCrack) {
		return true, nil
	}

	if status.String == string(StatusFailedToCap) {
		if !lastScan.Valid {
			return false, nil
		}
		return time.Since(lastScan.Time) < RetryDelay, nil
	}

	return false, nil
}

func (d *Database) TargetExists(bssid string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scanned WHERE bssid = ?", bssid).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type FilterParams struct {
	Search     string
	Encryption string
	Channel    string
	Status     string
	Page       int
	PerPage    int
}

type PaginatedResult struct {
	Targets    []map[string]interface{}
	TotalCount int
	Page       int
	PerPage    int
	TotalPages int
}

func (d *Database) GetAllTargets() ([]map[string]interface{}, error) {
	return d.GetTargetsWithFilters(FilterParams{Page: 1, PerPage: 1000})
}

func (d *Database) GetTargetsWithFilters(params FilterParams) ([]map[string]interface{}, error) {
	result, err := d.GetPaginatedTargets(params)
	if err != nil {
		return nil, err
	}
	return result.Targets, nil
}

func (d *Database) GetPaginatedTargets(params FilterParams) (*PaginatedResult, error) {
	if params.PerPage == 0 {
		params.PerPage = 20
	}
	if params.Page == 0 {
		params.Page = 1
	}

	whereClause := "1=1"
	args := []interface{}{}

	if params.Search != "" {
		whereClause += " AND (essid LIKE ? OR bssid LIKE ?)"
		searchTerm := "%" + params.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}
	if params.Encryption != "" {
		whereClause += " AND encryption = ?"
		args = append(args, params.Encryption)
	}
	if params.Channel != "" {
		whereClause += " AND channel = ?"
		args = append(args, params.Channel)
	}
	if params.Status != "" {
		whereClause += " AND status = ?"
		args = append(args, params.Status)
	}

	var totalCount int
	countQuery := "SELECT COUNT(*) FROM scanned WHERE " + whereClause
	err := d.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	totalPages := (totalCount + params.PerPage - 1) / params.PerPage

	offset := (params.Page - 1) * params.PerPage
	query := `
		SELECT bssid, essid, signal, channel, encryption, handshake_path, status, last_scan, cracked_password 
		FROM scanned 
		WHERE ` + whereClause + `
		ORDER BY last_scan DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, params.PerPage, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []map[string]interface{}
	for rows.Next() {
		var bssid, essid, channel, encryption, handshakePath, status string
		var signal int
		var lastScan sql.NullTime
		var crackedPassword sql.NullString

		err := rows.Scan(&bssid, &essid, &signal, &channel, &encryption, &handshakePath, &status, &lastScan, &crackedPassword)
		if err != nil {
			continue
		}

		target := map[string]interface{}{
			"bssid":         bssid,
			"essid":         essid,
			"signal":        signal,
			"channel":       channel,
			"encryption":    encryption,
			"handshakePath": handshakePath,
			"status":        status,
		}

		if lastScan.Valid {
			target["lastScan"] = lastScan.Time.Format("2006-01-02 15:04:05")
		} else {
			target["lastScan"] = ""
		}

		if crackedPassword.Valid {
			target["crackedPassword"] = crackedPassword.String
		} else {
			target["crackedPassword"] = ""
		}

		targets = append(targets, target)
	}

	return &PaginatedResult{
		Targets:    targets,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (d *Database) GetUniqueEncryptions() ([]string, error) {
	rows, err := d.db.Query("SELECT DISTINCT encryption FROM scanned WHERE encryption != '' ORDER BY encryption")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var encryptions []string
	for rows.Next() {
		var encryption string
		if err := rows.Scan(&encryption); err == nil {
			encryptions = append(encryptions, encryption)
		}
	}
	return encryptions, nil
}

func (d *Database) GetUniqueChannels() ([]string, error) {
	rows, err := d.db.Query("SELECT DISTINCT channel FROM scanned WHERE channel != '' ORDER BY CAST(channel AS INTEGER)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var channel string
		if err := rows.Scan(&channel); err == nil {
			channels = append(channels, channel)
		}
	}
	return channels, nil
}

func (d *Database) GetUniqueStatuses() ([]string, error) {
	rows, err := d.db.Query("SELECT DISTINCT status FROM scanned WHERE status != '' ORDER BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []string
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err == nil {
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}

func (d *Database) ResetScanningStatus() error {
	_, err := d.db.Exec(`
		UPDATE scanned 
		SET status = ? 
		WHERE status = ?`,
		string(StatusDiscovered),
		string(StatusScanning),
	)
	return err
}

func (d *Database) GetTarget(bssid string) map[string]interface{} {
	var (
		b, essid, channel, encryption, status, handshakePath, lastScan, crackedPassword string
		signal                                                                          int
	)

	err := d.db.QueryRow(`
		SELECT bssid, essid, channel, signal, encryption, status, handshake_path, last_scan, cracked_password
		FROM scanned
		WHERE bssid = ?
	`, bssid).Scan(&b, &essid, &channel, &signal, &encryption, &status, &handshakePath, &lastScan, &crackedPassword)

	if err != nil {
		return nil
	}

	return map[string]interface{}{
		"bssid":           b,
		"essid":           essid,
		"channel":         channel,
		"signal":          signal,
		"encryption":      encryption,
		"status":          status,
		"handshakePath":   handshakePath,
		"lastScan":        lastScan,
		"crackedPassword": crackedPassword,
	}
}

func (d *Database) DeleteTarget(bssid string) error {
	_, err := d.db.Exec("DELETE FROM scanned WHERE bssid = ?", bssid)
	return err
}
