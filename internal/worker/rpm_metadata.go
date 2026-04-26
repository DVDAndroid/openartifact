// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"sync"
	"time"

	sq "github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/format/base"
	formatrpm "github.com/openartifact/openartifact/internal/format/rpm"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/gpg"
	"github.com/openartifact/openartifact/pkg/logger"
	pkgrpm "github.com/openartifact/openartifact/pkg/rpm"
)

type RPMetadataWorker struct {
	db        sq.Database
	blobStore storage.BlobStore
	signer    gpg.Signer
	log       *logger.Logger
	reg       *metrics.Registry

	mu       sync.Mutex
	timers   map[string]*time.Timer
	debounce time.Duration
}

func NewRPMetadataWorker(db sq.Database, blobStore storage.BlobStore, signer gpg.Signer, log *logger.Logger, reg *metrics.Registry) *RPMetadataWorker {
	return &RPMetadataWorker{
		db:        db,
		blobStore: blobStore,
		signer:    signer,
		log:       log,
		reg:       reg,
		timers:    make(map[string]*time.Timer),
		debounce:  500 * time.Millisecond,
	}
}

func (w *RPMetadataWorker) NotifyStale(repoID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[repoID]; ok {
		t.Stop()
	}

	var t *time.Timer

	t = time.AfterFunc(w.debounce, func() {
		defer func() {
			if r := recover(); r != nil {
				w.log.Errorw("panic in goroutine", "panic", r)
			}
		}()
		w.log.Infow("metadata regeneration triggered", "repo_id", repoID)

		start := time.Now()

		err := w.regenerate(context.Background(), repoID)
		if w.reg != nil {
			w.reg.MetadataRegenerationDurationSeconds.WithLabelValues("rpm").Observe(time.Since(start).Seconds())
			w.reg.MetadataRegenerationsTotal.WithLabelValues("rpm").Inc()
		}

		if err != nil {
			w.log.Errorw("metadata regeneration failed", "repo_id", repoID, "error", err)
		}

		w.mu.Lock()
		if w.timers[repoID] == t {
			delete(w.timers, repoID)
		}
		w.mu.Unlock()
	})
	w.timers[repoID] = t
}

func (w *RPMetadataWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for id, t := range w.timers {
		t.Stop()
		delete(w.timers, id)
	}
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (w *RPMetadataWorker) regenerate(ctx context.Context, repoID string) error {
	conn := w.db.GetDB()

	var (
		baseArch      string
		metadataStale bool
	)

	err := conn.QueryRowContext(ctx, `
		SELECT base_arch, metadata_stale
		FROM rpm_repos
		WHERE repository_id = $1
	`, repoID).Scan(&baseArch, &metadataStale)
	if err != nil {
		return fmt.Errorf("rpm worker: querying rpm_repos: %w", err)
	}

	if !metadataStale {
		w.log.Debugw("metadata not stale, skipping", "repo_id", repoID)
		return nil
	}

	var locked bool

	err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtext($1))", repoID).Scan(&locked)
	if err != nil || !locked {
		return fmt.Errorf("rpm worker: could not acquire advisory lock for %s", repoID) //nolint:err113 // dynamic error with runtime context
	}

	defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext($1))", repoID) }()

	var staleMarker time.Time

	err = conn.QueryRowContext(ctx, `
		SELECT metadata_stale, updated_at FROM rpm_repos WHERE repository_id = $1
	`, repoID).Scan(&metadataStale, &staleMarker)
	if err != nil || !metadataStale {
		return nil
	}

	tx, err := w.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("rpm worker: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT rp.name, rp.version, COALESCE(rp.release, ''), rp.epoch, COALESCE(rp.arch, ''),
		       COALESCE(rp.summary, ''), COALESCE(rp.description, ''), COALESCE(rp.license, ''),
		       COALESCE(rp.url, ''), COALESCE(rp.vendor, ''), COALESCE(rp.group_name, ''),
		       COALESCE(rp.packager, ''), COALESCE(rp.source_rpm, ''), COALESCE(rp.build_host, ''),
		       COALESCE(EXTRACT(EPOCH FROM rp.build_time)::bigint, 0),
		       COALESCE(rp.package_size, 0), COALESCE(rp.installed_size, 0), COALESCE(rp.archive_size, 0),
		       b.sha256
		FROM rpm_packages rp
		JOIN blobs b ON b.id = rp.blob_id
		WHERE rp.repository_id = $1
	`, repoID)
	if err != nil {
		return fmt.Errorf("rpm worker: querying components: %w", err)
	}
	defer func() { base.CloseLog(rows, w.log) }()

	var packages []pkgrpm.Package

	for rows.Next() {
		var pkg pkgrpm.Package

		err := rows.Scan(
			&pkg.Name, &pkg.Version, &pkg.Release, &pkg.Epoch, &pkg.Arch,
			&pkg.Summary, &pkg.Description, &pkg.License, &pkg.URL,
			&pkg.Vendor, &pkg.Group, &pkg.Packager, &pkg.SourceRPM, &pkg.BuildHost,
			&pkg.BuildTime, &pkg.PackageSize, &pkg.InstalledSize, &pkg.ArchiveSize, &pkg.SHA256,
		)
		if err != nil {
			return fmt.Errorf("rpm worker: scanning component row: %w", err)
		}

		packages = append(packages, pkg)
	}

	revision := strconv.FormatInt(time.Now().Unix(), 10)
	ts := time.Now().Unix()

	primaryXML, err := formatrpm.GeneratePrimaryXML(packages, "")
	if err != nil {
		return fmt.Errorf("rpm worker: generating primary.xml: %w", err)
	}

	filelistsXML, err := formatrpm.GenerateFilelistsXML(packages)
	if err != nil {
		return fmt.Errorf("rpm worker: generating filelists.xml: %w", err)
	}

	otherXML, err := formatrpm.GenerateOtherXML(packages)
	if err != nil {
		return fmt.Errorf("rpm worker: generating other.xml: %w", err)
	}

	primaryCompressed, err := formatrpm.CompressMetadata(primaryXML)
	if err != nil {
		return fmt.Errorf("rpm worker: compressing primary.xml: %w", err)
	}

	filelistsCompressed, err := formatrpm.CompressMetadata(filelistsXML)
	if err != nil {
		return fmt.Errorf("rpm worker: compressing filelists.xml: %w", err)
	}

	otherCompressed, err := formatrpm.CompressMetadata(otherXML)
	if err != nil {
		return fmt.Errorf("rpm worker: compressing other.xml: %w", err)
	}

	entries := []formatrpm.RepomdData{
		{
			Type: "primary", Location: fmt.Sprintf("repodata/%s-primary.xml.gz", primaryCompressed.GzSHA256),
			Checksum: primaryCompressed.GzSHA256, ChecksumType: "sha256",
			Timestamp: ts, Size: primaryCompressed.GzSize, OpenSize: primaryCompressed.OpenSize,
		},
		{
			Type: "filelists", Location: fmt.Sprintf("repodata/%s-filelists.xml.gz", filelistsCompressed.GzSHA256),
			Checksum: filelistsCompressed.GzSHA256, ChecksumType: "sha256",
			Timestamp: ts, Size: filelistsCompressed.GzSize, OpenSize: filelistsCompressed.OpenSize,
		},
		{
			Type: "other", Location: fmt.Sprintf("repodata/%s-other.xml.gz", otherCompressed.GzSHA256),
			Checksum: otherCompressed.GzSHA256, ChecksumType: "sha256",
			Timestamp: ts, Size: otherCompressed.GzSize, OpenSize: otherCompressed.OpenSize,
		},
	}

	// Check for comps.xml and add group entry if it exists
	compsMeta, err := w.db.GetRepoMetadataByPath(ctx, repoID, "comps.xml")
	if err == nil && compsMeta != nil {
		entries = append(entries, formatrpm.RepomdData{
			Type: "group", Location: "repodata/comps.xml",
			Checksum: compsMeta.BlobSHA256, ChecksumType: "sha256",
			Timestamp: ts, Size: compsMeta.BlobSize, OpenSize: 0,
		})
	}

	repomdXML, err := formatrpm.GenerateRepomdXML(revision, entries)
	if err != nil {
		return fmt.Errorf("rpm worker: generating repomd.xml: %w", err)
	}

	blobs := map[string][]byte{
		fmt.Sprintf("repodata/%s-primary.xml.gz", primaryCompressed.GzSHA256):     primaryCompressed.GzipData,
		fmt.Sprintf("repodata/%s-primary.xml.xz", primaryCompressed.XZSHA256):     primaryCompressed.XZData,
		fmt.Sprintf("repodata/%s-filelists.xml.gz", filelistsCompressed.GzSHA256): filelistsCompressed.GzipData,
		fmt.Sprintf("repodata/%s-filelists.xml.xz", filelistsCompressed.XZSHA256): filelistsCompressed.XZData,
		fmt.Sprintf("repodata/%s-other.xml.gz", otherCompressed.GzSHA256):         otherCompressed.GzipData,
		fmt.Sprintf("repodata/%s-other.xml.xz", otherCompressed.XZSHA256):         otherCompressed.XZData,
		"repodata/repomd.xml": repomdXML,
	}

	if w.signer != nil {
		signature, signErr := formatrpm.SignRepomdXML(w.signer, repomdXML)
		if signErr != nil {
			w.log.Warnw("failed to sign repomd.xml", "error", signErr)
		} else {
			blobs["repodata/repomd.xml.asc"] = signature
		}

		publicKey, keyErr := formatrpm.PublicKey(w.signer)
		if keyErr != nil {
			w.log.Warnw("failed to export public key", "error", keyErr)
		} else {
			blobs["public.key"] = publicKey
		}
	}

	for path, data := range blobs {
		checksum := fmt.Sprintf("%x", sha256.Sum256(data))

		_, err := w.blobStore.Put(ctx, bytes.NewReader(data), map[string]string{
			"path":      path,
			"format":    "rpm",
			"repo_id":   repoID,
			"base_arch": baseArch,
			"sha256":    checksum,
		})
		if err != nil {
			return fmt.Errorf("rpm worker: storing blob %s: %w", path, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO repository_metadata (id, repository_id, path, blob_sha256, blob_size, last_generated)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, now())
			ON CONFLICT (repository_id, path) DO UPDATE
			SET blob_sha256 = $3, blob_size = $4, last_generated = now()
		`, repoID, path, checksum, int64(len(data)))
		if err != nil {
			return fmt.Errorf("rpm worker: updating repository_metadata for %s: %w", path, err)
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE rpm_repos SET metadata_stale = false
		WHERE repository_id = $1 AND updated_at = $2
	`, repoID, staleMarker)
	if err != nil {
		return fmt.Errorf("rpm worker: clearing stale flag: %w", err)
	}

	if n, raErr := result.RowsAffected(); raErr == nil && n == 0 {
		// A concurrent upload marked the repo stale again (bumping
		// updated_at) after we captured staleMarker; leave metadata_stale
		// = true rather than clobbering it. That upload's own
		// MarkRepoStale + NotifyStale call already schedules a follow-up
		// regen that will pick up its data.
		w.log.Infow("metadata_stale changed concurrently during regeneration; leaving stale for follow-up regen",
			"repo_id", repoID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("rpm worker: commit: %w", err)
	}

	w.log.Infow("metadata regeneration complete", "repo_id", repoID, "packages", len(packages))

	return nil
}
