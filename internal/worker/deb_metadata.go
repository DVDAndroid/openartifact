// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	sq "github.com/openartifact/openartifact/internal/database"
	formatdeb "github.com/openartifact/openartifact/internal/format/deb"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/gpg"
	"github.com/openartifact/openartifact/pkg/logger"
)

type DebMetadataWorker struct {
	db        sq.Database
	blobStore storage.BlobStore
	signer    gpg.Signer
	log       *logger.Logger

	mu       sync.Mutex
	timers   map[string]*time.Timer
	debounce time.Duration
}

func NewDebMetadataWorker(db sq.Database, blobStore storage.BlobStore, signer gpg.Signer, log *logger.Logger) *DebMetadataWorker {
	return &DebMetadataWorker{
		db:        db,
		blobStore: blobStore,
		signer:    signer,
		log:       log,
		timers:    make(map[string]*time.Timer),
		debounce:  500 * time.Millisecond,
	}
}

func (w *DebMetadataWorker) NotifyStale(repoID string) {
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
		w.log.Infow("deb metadata regeneration triggered", "repo_id", repoID)

		if err := w.regenerate(context.Background(), repoID); err != nil {
			w.log.Errorw("deb metadata regeneration failed", "repo_id", repoID, "error", err)
		}

		w.mu.Lock()
		if w.timers[repoID] == t {
			delete(w.timers, repoID)
		}
		w.mu.Unlock()
	})
	w.timers[repoID] = t
}

func (w *DebMetadataWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for id, t := range w.timers {
		t.Stop()
		delete(w.timers, id)
	}
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (w *DebMetadataWorker) regenerate(ctx context.Context, repoID string) error {
	conn := w.db.GetDB()

	var locked bool

	err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtext($1))", repoID).Scan(&locked)
	if err != nil || !locked {
		return fmt.Errorf("deb worker: could not acquire advisory lock for %s", repoID) //nolint:err113 // dynamic error with runtime context
	}

	defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext($1))", repoID) }()

	tx, err := w.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("deb worker: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get all distributions for this repository
	distributions, err := w.db.GetDebianDistributionsByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("deb worker: querying distributions: %w", err)
	}

	blobs := make(map[string][]byte)

	for _, dist := range distributions {
		// Reset fileInfos for each distribution
		fileInfos := make([]formatdeb.FileInfo, 0)

		releaseGen := formatdeb.NewReleaseFileGenerator("openartifact", "openartifact", dist.Name, dist.Name)

		// Get all components for this distribution
		components, err := w.db.GetDebianComponentsByDist(ctx, dist.ID)
		if err != nil {
			return fmt.Errorf("deb worker: querying components for dist %s: %w", dist.Name, err)
		}

		// Get all architectures for this distribution
		architectures, err := w.db.GetDebianArchitecturesByDist(ctx, dist.ID)
		if err != nil {
			return fmt.Errorf("deb worker: querying architectures for dist %s: %w", dist.Name, err)
		}

		// Build map of component/arch -> packages generator
		packagesGens := make(map[string]map[string]*formatdeb.PackagesFileGenerator)

		// For each component and architecture combination, get packages
		for _, comp := range components {
			for _, arch := range architectures {
				// Query packages for this dist/component/arch combination
				packages, err := w.db.GetDebianPackagesByDistComp(ctx, repoID, dist.Name, comp.Name, arch.Name)
				if err != nil {
					return fmt.Errorf("deb worker: querying packages for %s/%s/%s: %w", dist.Name, comp.Name, arch.Name, err)
				}

				if len(packages) == 0 {
					continue
				}

				// Initialize nested map if needed
				if _, ok := packagesGens[comp.Name]; !ok {
					packagesGens[comp.Name] = make(map[string]*formatdeb.PackagesFileGenerator)
				}

				if _, ok := packagesGens[comp.Name][arch.Name]; !ok {
					packagesGens[comp.Name][arch.Name] = formatdeb.NewPackagesFileGenerator()
					releaseGen.AddComponent(comp.Name)
					releaseGen.AddArchitecture(arch.Name)
				}

				// Add each package to the Packages file
				for _, pkg := range packages {
					// Build proper Debian control format entry
					entry := fmt.Sprintf("Package: %s\nVersion: %s\nArchitecture: %s\nInstalled-Size: %d\nFilename: %s\nSize: %d\nSHA256: %s\nDescription: %s\n\n",
						pkg.Name, pkg.Version, pkg.Arch, pkg.PackageSize/1024, pkg.Filename, pkg.PackageSize, pkg.SHA256, pkg.Summary)

					packagesGens[comp.Name][arch.Name].AddEntry(entry)
				}
			}
		}

		// Generate Packages files for each component/arch pair
		for comp, archMap := range packagesGens {
			for arch, gen := range archMap {
				content := gen.Generate()
				packagesPath := fmt.Sprintf("dists/%s/%s/binary-%s/Packages", dist.Name, comp, arch)
				blobs[packagesPath] = []byte(content)

				// Add checksums for plain Packages file
				md5sum, sha1sum, sha256sum := formatdeb.CalculateChecksums([]byte(content))
				fileInfos = append(fileInfos, formatdeb.FileInfo{
					Filename: packagesPath,
					Size:     int64(len(content)),
					MD5sum:   md5sum,
					SHA1:     sha1sum,
					SHA256:   sha256sum,
				})

				gzipped, err := gen.GenerateGzipped()
				if err != nil {
					return fmt.Errorf("deb worker: compressing packages for %s/%s/%s: %w", dist.Name, comp, arch, err)
				}

				packagesGzPath := fmt.Sprintf("dists/%s/%s/binary-%s/Packages.gz", dist.Name, comp, arch)
				blobs[packagesGzPath] = gzipped

				md5sum, sha1sum, sha256sum = formatdeb.CalculateChecksums(gzipped)
				fileInfos = append(fileInfos, formatdeb.FileInfo{
					Filename: packagesGzPath,
					Size:     int64(len(gzipped)),
					MD5sum:   md5sum,
					SHA1:     sha1sum,
					SHA256:   sha256sum,
				})

				xzipped, err := gen.GenerateXZipped()
				if err != nil {
					return fmt.Errorf("deb worker: xz compressing packages for %s/%s/%s: %w", dist.Name, comp, arch, err)
				}

				packagesXzPath := fmt.Sprintf("dists/%s/%s/binary-%s/Packages.xz", dist.Name, comp, arch)
				blobs[packagesXzPath] = xzipped

				md5sum, sha1sum, sha256sum = formatdeb.CalculateChecksums(xzipped)
				fileInfos = append(fileInfos, formatdeb.FileInfo{
					Filename: packagesXzPath,
					Size:     int64(len(xzipped)),
					MD5sum:   md5sum,
					SHA1:     sha1sum,
					SHA256:   sha256sum,
				})
			}
		}

		releaseContent := releaseGen.GenerateRelease(fileInfos)
		releasePath := fmt.Sprintf("dists/%s/Release", dist.Name)
		blobs[releasePath] = []byte(releaseContent)

		if w.signer != nil {
			signer := formatdeb.NewReleaseSigner(w.signer)

			signed, err := signer.SignRelease([]byte(releaseContent))
			if err != nil {
				w.log.Warnw("failed to sign release", "dist", dist.Name, "error", err)
			} else {
				inReleasePath := fmt.Sprintf("dists/%s/InRelease", dist.Name)
				releaseGPGPath := fmt.Sprintf("dists/%s/Release.gpg", dist.Name)
				blobs[inReleasePath] = signed.InRelease
				blobs[releaseGPGPath] = signed.ReleaseGPG
			}
		}
	}

	for path, data := range blobs {
		checksum := fmt.Sprintf("%x", sha256.Sum256(data))

		_, err = w.blobStore.Put(ctx, bytes.NewReader(data), map[string]string{
			"path":    path,
			"format":  "deb",
			"repo_id": repoID,
			"sha256":  checksum,
		})
		if err != nil {
			return fmt.Errorf("deb worker: storing blob %s: %w", path, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO repository_metadata (id, repository_id, path, blob_sha256, blob_size, last_generated)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, now())
			ON CONFLICT (repository_id, path) DO UPDATE
			SET blob_sha256 = $3, blob_size = $4, last_generated = now()
		`, repoID, path, checksum, int64(len(data)))
		if err != nil {
			return fmt.Errorf("deb worker: updating repo_metadata for %s: %w", path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("deb worker: commit: %w", err)
	}

	w.log.Infow("deb metadata regeneration complete", "repo_id", repoID, "distributions", len(distributions))

	return nil
}
