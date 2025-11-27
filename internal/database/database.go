package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/victor/stormindexer/internal/models"
)

type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates the necessary tables
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS indexes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		root_path TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		last_sync DATETIME,
		machine_id TEXT NOT NULL,
		total_files INTEGER DEFAULT 0,
		total_size INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL,
		relative_path TEXT NOT NULL,
		size INTEGER NOT NULL,
		mod_time DATETIME NOT NULL,
		checksum TEXT,
		index_id TEXT NOT NULL,
		last_scanned DATETIME NOT NULL,
		is_directory INTEGER NOT NULL DEFAULT 0,
		UNIQUE(path, index_id),
		FOREIGN KEY(index_id) REFERENCES indexes(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
	CREATE INDEX IF NOT EXISTS idx_files_index_id ON files(index_id);
	CREATE INDEX IF NOT EXISTS idx_files_checksum ON files(checksum);
	CREATE INDEX IF NOT EXISTS idx_files_relative_path ON files(relative_path);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// CreateIndex creates a new index entry
func (db *DB) CreateIndex(index *models.Index) error {
	query := `
	INSERT INTO indexes (id, name, root_path, created_at, last_sync, machine_id, total_files, total_size)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query, index.ID, index.Name, index.RootPath, index.CreatedAt, index.LastSync, index.MachineID, index.TotalFiles, index.TotalSize)
	return err
}

// GetIndex retrieves an index by ID
func (db *DB) GetIndex(indexID string) (*models.Index, error) {
	query := `
	SELECT id, name, root_path, created_at, last_sync, machine_id, total_files, total_size
	FROM indexes
	WHERE id = ?
	`
	index := &models.Index{}
	var createdAt, lastSync string
	err := db.conn.QueryRow(query, indexID).Scan(
		&index.ID, &index.Name, &index.RootPath, &createdAt, &lastSync,
		&index.MachineID, &index.TotalFiles, &index.TotalSize,
	)
	if err != nil {
		return nil, err
	}

	index.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastSync != "" {
		index.LastSync, _ = time.Parse(time.RFC3339, lastSync)
	}

	return index, nil
}

// FindIndexByNameOrID finds an index by exact name match or partial ID match
func (db *DB) FindIndexByNameOrID(identifier string) (*models.Index, error) {
	// First try exact ID match
	index, err := db.GetIndex(identifier)
	if err == nil {
		return index, nil
	}

	// Then try exact name match
	query := `
	SELECT id, name, root_path, created_at, last_sync, machine_id, total_files, total_size
	FROM indexes
	WHERE name = ?
	LIMIT 1
	`
	index = &models.Index{}
	var createdAt, lastSync string
	err = db.conn.QueryRow(query, identifier).Scan(
		&index.ID, &index.Name, &index.RootPath, &createdAt, &lastSync,
		&index.MachineID, &index.TotalFiles, &index.TotalSize,
	)
	if err == nil {
		index.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if lastSync != "" {
			index.LastSync, _ = time.Parse(time.RFC3339, lastSync)
		}
		return index, nil
	}

	// Finally try partial ID match (at least 8 characters)
	if len(identifier) >= 8 {
		query = `
		SELECT id, name, root_path, created_at, last_sync, machine_id, total_files, total_size
		FROM indexes
		WHERE id LIKE ?
		LIMIT 1
		`
		index = &models.Index{}
		err = db.conn.QueryRow(query, identifier+"%").Scan(
			&index.ID, &index.Name, &index.RootPath, &createdAt, &lastSync,
			&index.MachineID, &index.TotalFiles, &index.TotalSize,
		)
		if err == nil {
			index.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
			if lastSync != "" {
				index.LastSync, _ = time.Parse(time.RFC3339, lastSync)
			}
			return index, nil
		}
	}

	return nil, fmt.Errorf("index not found: %s", identifier)
}

// ListIndexes returns all indexes
func (db *DB) ListIndexes() ([]*models.Index, error) {
	query := `
	SELECT id, name, root_path, created_at, last_sync, machine_id, total_files, total_size
	FROM indexes
	ORDER BY created_at DESC
	`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []*models.Index
	for rows.Next() {
		index := &models.Index{}
		var createdAt, lastSync string
		if err := rows.Scan(
			&index.ID, &index.Name, &index.RootPath, &createdAt, &lastSync,
			&index.MachineID, &index.TotalFiles, &index.TotalSize,
		); err != nil {
			return nil, err
		}

		index.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if lastSync != "" {
			index.LastSync, _ = time.Parse(time.RFC3339, lastSync)
		}

		indexes = append(indexes, index)
	}

	return indexes, rows.Err()
}

// UpsertFile inserts or updates a file entry
func (db *DB) UpsertFile(file *models.FileEntry) error {
	query := `
	INSERT INTO files (path, relative_path, size, mod_time, checksum, index_id, last_scanned, is_directory)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path, index_id) DO UPDATE SET
		size = excluded.size,
		mod_time = excluded.mod_time,
		checksum = excluded.checksum,
		last_scanned = excluded.last_scanned,
		is_directory = excluded.is_directory
	`
	_, err := db.conn.Exec(query,
		file.Path, file.RelativePath, file.Size, file.ModTime, file.Checksum,
		file.IndexID, file.LastScanned, file.IsDirectory,
	)
	return err
}

// GetFile retrieves a file by path and index ID
func (db *DB) GetFile(path, indexID string) (*models.FileEntry, error) {
	query := `
	SELECT id, path, relative_path, size, mod_time, checksum, index_id, last_scanned, is_directory
	FROM files
	WHERE path = ? AND index_id = ?
	`
	file := &models.FileEntry{}
	var modTime, lastScanned string
	err := db.conn.QueryRow(query, path, indexID).Scan(
		&file.ID, &file.Path, &file.RelativePath, &file.Size, &modTime,
		&file.Checksum, &file.IndexID, &lastScanned, &file.IsDirectory,
	)
	if err != nil {
		return nil, err
	}

	file.ModTime, _ = time.Parse(time.RFC3339, modTime)
	file.LastScanned, _ = time.Parse(time.RFC3339, lastScanned)

	return file, nil
}

// ListFiles returns all files for a given index
func (db *DB) ListFiles(indexID string) ([]*models.FileEntry, error) {
	query := `
	SELECT id, path, relative_path, size, mod_time, checksum, index_id, last_scanned, is_directory
	FROM files
	WHERE index_id = ?
	ORDER BY path
	`
	rows, err := db.conn.Query(query, indexID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.FileEntry
	for rows.Next() {
		file := &models.FileEntry{}
		var modTime, lastScanned string
		if err := rows.Scan(
			&file.ID, &file.Path, &file.RelativePath, &file.Size, &modTime,
			&file.Checksum, &file.IndexID, &lastScanned, &file.IsDirectory,
		); err != nil {
			return nil, err
		}

		file.ModTime, _ = time.Parse(time.RFC3339, modTime)
		file.LastScanned, _ = time.Parse(time.RFC3339, lastScanned)

		files = append(files, file)
	}

	return files, rows.Err()
}

// DeleteFile removes a file from the index
func (db *DB) DeleteFile(path, indexID string) error {
	query := `DELETE FROM files WHERE path = ? AND index_id = ?`
	_, err := db.conn.Exec(query, path, indexID)
	return err
}

// DeleteIndex removes an index and all its files (CASCADE deletes files automatically)
func (db *DB) DeleteIndex(indexID string) error {
	query := `DELETE FROM indexes WHERE id = ?`
	result, err := db.conn.Exec(query, indexID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("index not found: %s", indexID)
	}
	
	return nil
}

// UpdateIndexStats updates the statistics for an index
func (db *DB) UpdateIndexStats(indexID string) error {
	query := `
	UPDATE indexes
	SET total_files = (SELECT COUNT(*) FROM files WHERE index_id = ?),
		total_size = (SELECT COALESCE(SUM(size), 0) FROM files WHERE index_id = ? AND is_directory = 0),
		last_sync = ?
	WHERE id = ?
	`
	_, err := db.conn.Exec(query, indexID, indexID, time.Now(), indexID)
	return err
}

// FindFilesByChecksum finds files with the same checksum across different indexes
func (db *DB) FindFilesByChecksum(checksum string) ([]*models.FileEntry, error) {
	query := `
	SELECT id, path, relative_path, size, mod_time, checksum, index_id, last_scanned, is_directory
	FROM files
	WHERE checksum = ? AND checksum != ''
	ORDER BY index_id, path
	`
	rows, err := db.conn.Query(query, checksum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.FileEntry
	for rows.Next() {
		file := &models.FileEntry{}
		var modTime, lastScanned string
		if err := rows.Scan(
			&file.ID, &file.Path, &file.RelativePath, &file.Size, &modTime,
			&file.Checksum, &file.IndexID, &lastScanned, &file.IsDirectory,
		); err != nil {
			return nil, err
		}

		file.ModTime, _ = time.Parse(time.RFC3339, modTime)
		file.LastScanned, _ = time.Parse(time.RFC3339, lastScanned)

		files = append(files, file)
	}

	return files, rows.Err()
}


// FindOptions represents search criteria for finding files
type FindOptions struct {
	NamePattern      string
	DirectoryPattern string
	Checksum         string
	MinSize          int64
	MaxSize          int64
	IndexIDs         []string
	OnlyDuplicates   bool
	ModifiedSince    *time.Time
	ModifiedUntil    *time.Time
	FileType         string // "file", "dir", "directory", "all"
}

// FileWithIndex represents a file entry with index metadata
type FileWithIndex struct {
	*models.FileEntry
	IndexName string
	IndexPath string
}

// FindFiles searches for files across all indexes based on the provided options
func (db *DB) FindFiles(opts FindOptions) ([]*FileWithIndex, error) {
	var conditions []string
	var args []interface{}

	// Build WHERE clause conditions
	if opts.NamePattern != "" {
		// Convert shell-style wildcards to SQL LIKE patterns
		pattern := convertPatternToLike(opts.NamePattern)
		conditions = append(conditions, "f.relative_path LIKE ?")
		args = append(args, pattern)
	}

	if opts.DirectoryPattern != "" {
		dirPattern := convertPatternToLike(opts.DirectoryPattern)
		// Match directory name anywhere in path
		conditions = append(conditions, `(
			f.relative_path LIKE ? || '/%'
			OR f.relative_path LIKE '%/' || ? || '/%'
			OR f.relative_path LIKE '%/' || ?
			OR f.relative_path LIKE ?
		)`)
		args = append(args, dirPattern, dirPattern, dirPattern, dirPattern)
	}

	if opts.Checksum != "" {
		conditions = append(conditions, "f.checksum = ?")
		args = append(args, opts.Checksum)
	}

	if opts.MinSize > 0 {
		conditions = append(conditions, "f.size >= ?")
		args = append(args, opts.MinSize)
	}

	if opts.MaxSize > 0 {
		conditions = append(conditions, "f.size <= ?")
		args = append(args, opts.MaxSize)
	}

	// File type filtering
	fileType := opts.FileType
	if fileType == "" {
		fileType = "all"
	}
	if fileType == "directory" {
		fileType = "dir"
	}
	if fileType == "file" {
		conditions = append(conditions, "f.is_directory = 0")
	} else if fileType == "dir" {
		conditions = append(conditions, "f.is_directory = 1")
	}
	// "all" doesn't add a condition

	if opts.ModifiedSince != nil {
		conditions = append(conditions, "f.mod_time >= ?")
		args = append(args, opts.ModifiedSince.Format(time.RFC3339))
	}

	if opts.ModifiedUntil != nil {
		conditions = append(conditions, "f.mod_time <= ?")
		args = append(args, opts.ModifiedUntil.Format(time.RFC3339))
	}

	if len(opts.IndexIDs) > 0 {
		placeholders := ""
		for i, id := range opts.IndexIDs {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		conditions = append(conditions, "f.index_id IN ("+placeholders+")")
	}

	// Handle duplicates filter
	if opts.OnlyDuplicates {
		conditions = append(conditions, `f.checksum IN (
			SELECT checksum 
			FROM files 
			WHERE checksum != '' 
			GROUP BY checksum 
			HAVING COUNT(*) > 1
		)`)
	}

	// Build query
	query := `
	SELECT f.id, f.path, f.relative_path, f.size, f.mod_time, f.checksum, 
	       f.index_id, f.last_scanned, f.is_directory,
	       i.name as index_name, i.root_path as index_path
	FROM files f
	JOIN indexes i ON f.index_id = i.id
	`

	if len(conditions) > 0 {
		query += "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY i.name, f.path"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var results []*FileWithIndex
	for rows.Next() {
		file := &models.FileEntry{}
		var modTime, lastScanned string
		var indexName, indexPath string

		err := rows.Scan(
			&file.ID, &file.Path, &file.RelativePath, &file.Size, &modTime,
			&file.Checksum, &file.IndexID, &lastScanned, &file.IsDirectory,
			&indexName, &indexPath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		file.ModTime, _ = time.Parse(time.RFC3339, modTime)
		file.LastScanned, _ = time.Parse(time.RFC3339, lastScanned)

		results = append(results, &FileWithIndex{
			FileEntry: file,
			IndexName: indexName,
			IndexPath: indexPath,
		})
	}

	return results, rows.Err()
}

// convertPatternToLike converts shell-style wildcards (*, ?) to SQL LIKE patterns
func convertPatternToLike(pattern string) string {
	// Escape SQL LIKE special characters first (need to escape backslash first)
	pattern = strings.ReplaceAll(pattern, `\`, `\\`)
	pattern = strings.ReplaceAll(pattern, `%`, `\%`)
	pattern = strings.ReplaceAll(pattern, `_`, `\_`)

	// Convert wildcards
	pattern = strings.ReplaceAll(pattern, `*`, `%`)
	pattern = strings.ReplaceAll(pattern, `?`, `_`)

	return pattern
}
