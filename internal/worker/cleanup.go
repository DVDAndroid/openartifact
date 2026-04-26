// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/logger"
)

// Sentinel errors for cleanup worker.
var (
	ErrRepositoryNotFound = errors.New("repository not found")
)

type CleanupWorker struct {
	db        database.Database
	blobStore storage.BlobStore
	log       *logger.Logger
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func NewCleanupWorker(db database.Database, blobStore storage.BlobStore, log *logger.Logger) *CleanupWorker {
	return &CleanupWorker{
		db:        db,
		blobStore: blobStore,
		log:       log,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (w *CleanupWorker) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				w.log.Errorw("panic in goroutine", "panic", r)
			}
		}()
		defer close(w.doneCh)

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				w.runCleanupCycle(ctx)
				cancel()
			case <-w.stopCh:
				return
			}
		}
	}()
}

func (w *CleanupWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh
}

func (w *CleanupWorker) runCleanupCycle(ctx context.Context) {
	policies, err := w.db.ListEnabledCleanupPolicies(ctx)
	if err != nil {
		w.log.Errorw("failed to list enabled cleanup policies", "error", err)
		return
	}

	for _, policy := range policies {
		if w.shouldRunPolicy(policy) {
			w.executePolicy(ctx, policy)
		}
	}
}

func (w *CleanupWorker) shouldRunPolicy(policy *models.CleanupPolicy) bool {
	if policy.LastRunAt == nil {
		return true
	}
	// Simple check: if last run was more than 24 hours ago, run again
	// In production, you'd parse the cron expression for more accuracy
	return time.Since(*policy.LastRunAt) > 24*time.Hour
}

func (w *CleanupWorker) executePolicy(ctx context.Context, policy *models.CleanupPolicy) {
	conn := w.db.GetDB()

	// Acquire advisory lock to prevent concurrent execution
	var locked bool

	err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtext($1))", policy.ID).Scan(&locked)
	if err != nil || !locked {
		w.log.Debugw("could not acquire lock for cleanup policy", "policy_id", policy.ID)
		return
	}

	defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext($1))", policy.ID) }()

	deletedCount, runErr := w.runPolicy(ctx, policy)
	if err := w.db.UpdateCleanupPolicyRunResult(ctx, policy.ID, deletedCount, runErr); err != nil {
		w.log.Errorw("failed to update cleanup policy run result", "policy_id", policy.ID, "error", err)
	}

	if runErr != nil {
		w.log.Errorw("cleanup policy execution failed", "policy_id", policy.ID, "policy_name", policy.Name, "error", runErr)
	} else {
		w.log.Infow("cleanup policy executed successfully", "policy_id", policy.ID, "policy_name", policy.Name, "deleted", deletedCount)
	}
}

// formatTable maps a repository format to the single, self-contained table
// that stores its blob-bearing rows, for the generic cleanup-by-policy code
// below. Every table listed here has an `id` PK, a `repository_id` FK, a
// name-like column, a `blob_id`-shaped FK into the global blobs table, and a
// `created_at` timestamp.
type formatTable struct {
	table      string
	nameCol    string
	blobIDCol  string
	createdCol string
}

// cleanupFormatTable covers formats whose packages live in one table with a
// direct blob_id FK (e.g. rpm_packages/deb_packages). npm is intentionally not listed: its tarballs
// live in npm_versions, joined to npm_packages for name/repository_id — a
// different shape the single-table query below cannot express.
func cleanupFormatTable(format string) (formatTable, bool) {
	switch format {
	case "rpm":
		return formatTable{"rpm_packages", "name", "blob_id", "created_at"}, true
	case "deb":
		return formatTable{"deb_packages", "name", "blob_id", "created_at"}, true
	case "helm":
		return formatTable{"helm_charts", "name", "blob_id", "created_at"}, true
	case "pypi":
		return formatTable{"pypi_packages", "name", "blob_id", "created_at"}, true
	case "nuget":
		return formatTable{"nuget_packages", "package_id", "blob_id", "created_at"}, true
	case "go": // repositories.format value is "go", not "gomod" (chk_repo_format)
		return formatTable{"gomod_modules", "module_path", "zip_blob_id", "created_at"}, true
	default:
		return formatTable{}, false
	}
}

func (w *CleanupWorker) runPolicy(ctx context.Context, policy *models.CleanupPolicy) (int, error) {
	// Get the repository to determine its format.
	repo, err := w.db.GetRepositoryByID(ctx, policy.RepositoryID)
	if err != nil {
		return 0, fmt.Errorf("failed to get repository: %w", err)
	}

	if repo == nil {
		return 0, ErrRepositoryNotFound
	}

	tableInfo, ok := cleanupFormatTable(repo.Format)
	if !ok {
		return 0, fmt.Errorf("unsupported format for cleanup: %s", repo.Format) //nolint:err113 // dynamic error with runtime context
	}

	return w.deleteArtifacts(ctx, policy, tableInfo)
}

// cleanupCandidate is a row selected for deletion: its primary key (for the
// DELETE) and the blob_id it references (to resolve sha256 for SafeDeleteBlob).
type cleanupCandidate struct {
	id     string
	blobID string
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (w *CleanupWorker) deleteArtifacts(ctx context.Context, policy *models.CleanupPolicy, tableInfo formatTable) (int, error) {
	conn := w.db.GetDB()

	var (
		toDelete     []cleanupCandidate
		totalDeleted int
	)

	whereConditions := []string{"repository_id = $1"}
	args := []any{policy.RepositoryID}

	if policy.KeepSinceDays != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("%s < now() - interval '%d days'", tableInfo.createdCol, *policy.KeepSinceDays))
	}

	whereClause := "WHERE " + strings.Join(whereConditions, " AND ")

	switch {
	case policy.KeepLastN != nil:
		// Keep only the N most recently created rows per name; select the rest for deletion.
		//nolint:gosec // G201: query uses validated table/column names from internal constants
		query := fmt.Sprintf(`
			SELECT id, %s FROM (
				SELECT id, %s,
				       ROW_NUMBER() OVER (PARTITION BY %s ORDER BY %s DESC) AS rn
				FROM %s
				%s
			) ranked
			WHERE rn > $%d
		`, tableInfo.blobIDCol, tableInfo.blobIDCol, tableInfo.nameCol, tableInfo.createdCol,
			tableInfo.table, whereClause, len(args)+1)

		rows, err := conn.QueryContext(ctx, query, append(args, *policy.KeepLastN)...)
		if err != nil {
			return 0, fmt.Errorf("failed to query artifacts for deletion: %w", err)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var c cleanupCandidate
			if err := rows.Scan(&c.id, &c.blobID); err != nil {
				return 0, fmt.Errorf("failed to scan artifact row: %w", err)
			}

			toDelete = append(toDelete, c)
		}

		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("error iterating artifact rows: %w", err)
		}
	case policy.KeepSinceDays != nil:
		//nolint:gosec // G201: query uses validated table/column names from internal constants
		query := fmt.Sprintf(`SELECT id, %s FROM %s %s`, tableInfo.blobIDCol, tableInfo.table, whereClause)

		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return 0, fmt.Errorf("failed to query artifacts for deletion: %w", err)
		}

		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var c cleanupCandidate
			if err := rows.Scan(&c.id, &c.blobID); err != nil {
				return 0, fmt.Errorf("failed to scan artifact row: %w", err)
			}

			toDelete = append(toDelete, c)
		}

		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("error iterating artifact rows: %w", err)
		}
	}

	// Apply name pattern filter if specified.
	if policy.NamePattern != "" {
		nameRegex, err := regexp.Compile(policy.NamePattern)
		if err != nil {
			return 0, fmt.Errorf("invalid name pattern regex: %w", err)
		}

		filtered := toDelete[:0]

		for _, c := range toDelete {
			var name string

			nameErr := conn.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", tableInfo.nameCol, tableInfo.table), c.id).Scan(&name)
			if nameErr != nil && !errors.Is(nameErr, sql.ErrNoRows) {
				w.log.Warnw("failed to get artifact name", "id", c.id, "error", nameErr)
				continue
			}

			if nameRegex.MatchString(name) {
				filtered = append(filtered, c)
			}
		}

		toDelete = filtered
	}

	// Delete artifacts: the DB row first, then the underlying blob from
	// storage only if no other row still references it.
	for _, c := range toDelete {
		var bundleProtected bool

		checkErr := conn.QueryRowContext(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM release_bundle_items rbi
				JOIN blobs b ON b.sha256 = rbi.sha256
				WHERE b.id = $1
			)`, c.blobID,
		).Scan(&bundleProtected)
		if checkErr != nil {
			w.log.Warnw("failed to check bundle protection for blob", "blob_id", c.blobID, "error", checkErr)
			continue
		}

		if bundleProtected {
			w.log.Debugw("skipping cleanup: blob is referenced by a release bundle", "blob_id", c.blobID)
			continue
		}

		//nolint:gosec // G201: query uses validated table/column names from internal constants
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE id = $1", tableInfo.table)
		if _, err := conn.ExecContext(ctx, deleteQuery, c.id); err != nil {
			w.log.Warnw("failed to delete artifact from database", "id", c.id, "error", err)
			continue
		}

		totalDeleted++

		var sha256 string

		if err := conn.QueryRowContext(ctx, "SELECT sha256 FROM blobs WHERE id = $1", c.blobID).Scan(&sha256); err != nil {
			w.log.Warnw("failed to resolve blob sha256 for cleanup", "blob_id", c.blobID, "error", err)
			continue
		}

		if err := storage.SafeDeleteBlob(ctx, w.db, w.blobStore, sha256); err != nil {
			w.log.Warnw("failed to delete blob from storage", "sha256", sha256, "error", err)
		}
	}

	return totalDeleted, nil
}
