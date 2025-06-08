package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"nodeprobe/internal/domain"

	_ "github.com/mattn/go-sqlite3"
)

type Repository struct {
	db     *sql.DB
	dbPath string
}

func NewRepository(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	repo := &Repository{
		db:     db,
		dbPath: dbPath,
	}
	if err := repo.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return repo, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			fqdn TEXT NOT NULL,
			ip TEXT NOT NULL,
			discovered_by TEXT NOT NULL,
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true
		)`,
		`CREATE TABLE IF NOT EXISTS poll_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			poll_time DATETIME NOT NULL,
			success BOOLEAN NOT NULL,
			response_ms INTEGER,
			error TEXT,
			path_mtu INTEGER,
			FOREIGN KEY (node_id) REFERENCES nodes(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_is_active ON nodes(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_poll_results_node_id ON poll_results(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_poll_results_poll_time ON poll_results(poll_time)`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// NodeRepository implementation
func (r *Repository) GetAllNodes(ctx context.Context) ([]domain.Node, error) {
	query := `SELECT id, fqdn, ip, discovered_by, first_seen, last_seen, is_active 
			  FROM nodes ORDER BY first_seen ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	defer rows.Close()

	var nodes []domain.Node
	for rows.Next() {
		var node domain.Node
		err := rows.Scan(&node.ID, &node.FQDN, &node.IP, &node.DiscoveredBy,
			&node.FirstSeen, &node.LastSeen, &node.IsActive)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

func (r *Repository) GetNode(ctx context.Context, id string) (*domain.Node, error) {
	query := `SELECT id, fqdn, ip, discovered_by, first_seen, last_seen, is_active 
			  FROM nodes WHERE id = ?`

	var node domain.Node
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID, &node.FQDN, &node.IP, &node.DiscoveredBy,
		&node.FirstSeen, &node.LastSeen, &node.IsActive)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return &node, nil
}

func (r *Repository) CreateNode(ctx context.Context, node *domain.Node) error {
	query := `INSERT INTO nodes (id, fqdn, ip, discovered_by, first_seen, last_seen, is_active)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query, node.ID, node.FQDN, node.IP,
		node.DiscoveredBy, node.FirstSeen, node.LastSeen, node.IsActive)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	return nil
}

func (r *Repository) UpdateNode(ctx context.Context, node *domain.Node) error {
	query := `UPDATE nodes SET fqdn = ?, ip = ?, discovered_by = ?, 
			  first_seen = ?, last_seen = ?, is_active = ? WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, node.FQDN, node.IP, node.DiscoveredBy,
		node.FirstSeen, node.LastSeen, node.IsActive, node.ID)
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	return nil
}

func (r *Repository) DeleteNode(ctx context.Context, id string) error {
	query := `DELETE FROM nodes WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	return nil
}

func (r *Repository) GetActiveNodes(ctx context.Context) ([]domain.Node, error) {
	query := `SELECT id, fqdn, ip, discovered_by, first_seen, last_seen, is_active 
			  FROM nodes WHERE is_active = true ORDER BY first_seen ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active nodes: %w", err)
	}
	defer rows.Close()

	var nodes []domain.Node
	for rows.Next() {
		var node domain.Node
		err := rows.Scan(&node.ID, &node.FQDN, &node.IP, &node.DiscoveredBy,
			&node.FirstSeen, &node.LastSeen, &node.IsActive)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// PollRepository implementation
func (r *Repository) CreatePollResult(ctx context.Context, result *domain.PollResult) error {
	query := `INSERT INTO poll_results (node_id, poll_time, success, response_ms, error, path_mtu)
			  VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query, result.NodeID, result.PollTime,
		result.Success, result.ResponseMs, result.Error, result.PathMTU)
	if err != nil {
		return fmt.Errorf("failed to create poll result: %w", err)
	}

	return nil
}

func (r *Repository) GetPollResults(ctx context.Context, nodeID string, limit int) ([]domain.PollResult, error) {
	query := `SELECT id, node_id, poll_time, success, response_ms, error, path_mtu
			  FROM poll_results WHERE node_id = ? ORDER BY poll_time DESC LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query poll results: %w", err)
	}
	defer rows.Close()

	var results []domain.PollResult
	for rows.Next() {
		var result domain.PollResult
		var errorStr sql.NullString
		var pathMTU sql.NullInt64

		err := rows.Scan(&result.ID, &result.NodeID, &result.PollTime,
			&result.Success, &result.ResponseMs, &errorStr, &pathMTU)
		if err != nil {
			return nil, fmt.Errorf("failed to scan poll result: %w", err)
		}

		if errorStr.Valid {
			result.Error = errorStr.String
		}
		if pathMTU.Valid {
			result.PathMTU = int(pathMTU.Int64)
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

func (r *Repository) GetRecentPollResults(ctx context.Context, since time.Time) ([]domain.PollResult, error) {
	query := `SELECT id, node_id, poll_time, success, response_ms, error, path_mtu
			  FROM poll_results WHERE poll_time >= ? ORDER BY poll_time DESC`

	rows, err := r.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent poll results: %w", err)
	}
	defer rows.Close()

	var results []domain.PollResult
	for rows.Next() {
		var result domain.PollResult
		var errorStr sql.NullString
		var pathMTU sql.NullInt64

		err := rows.Scan(&result.ID, &result.NodeID, &result.PollTime,
			&result.Success, &result.ResponseMs, &errorStr, &pathMTU)
		if err != nil {
			return nil, fmt.Errorf("failed to scan poll result: %w", err)
		}

		if errorStr.Valid {
			result.Error = errorStr.String
		}
		if pathMTU.Valid {
			result.PathMTU = int(pathMTU.Int64)
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

func (r *Repository) GetDatabaseSize(ctx context.Context) (int64, error) {
	// Get database file info
	info, err := os.Stat(r.dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get database file info: %w", err)
	}

	return info.Size(), nil
}

func (r *Repository) CleanupOldResults(ctx context.Context, maxSizeMB int) error {
	// Check current database size
	currentSize, err := r.GetDatabaseSize(ctx)
	if err != nil {
		return err
	}

	maxSizeBytes := int64(maxSizeMB * 1024 * 1024)
	if currentSize <= maxSizeBytes {
		return nil // No cleanup needed
	}

	// Delete oldest poll results until we're under the limit
	query := `DELETE FROM poll_results WHERE id IN (
		SELECT id FROM poll_results ORDER BY poll_time ASC LIMIT 1000
	)`

	for currentSize > maxSizeBytes {
		result, err := r.db.ExecContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to cleanup old results: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			break // No more rows to delete
		}

		// Re-check database size
		currentSize, err = r.GetDatabaseSize(ctx)
		if err != nil {
			return err
		}
	}

	// Run VACUUM to reclaim space
	_, err = r.db.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	return nil
}
