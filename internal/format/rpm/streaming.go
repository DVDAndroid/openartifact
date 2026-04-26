// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

// StreamingGenerator generates RPM repository metadata on-the-fly from
// database queries, writing directly to an io.Writer without intermediate
// blob storage. This eliminates the metadata staleness window: clients always
// receive up-to-date metadata even before the background worker has run.
//
// Usage: call StreamRepomd or StreamPrimaryXML from HTTP handlers when no
// pre-generated blob exists in RepoMetadata.

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	db "github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	pkgrpm "github.com/openartifact/openartifact/pkg/rpm"
)

// StreamRepomd writes a minimal repomd.xml for the given repository directly
// to w. The generated repomd.xml references the streaming metadata endpoints
// (primary, filelists, other) without pre-computed checksums, which is valid
// for clients that do not verify repomd checksums (e.g. dnf with gpgcheck=0).
// For production use, the background worker generates a fully-checksummed version.
func StreamRepomd(ctx context.Context, database db.Database, repoID string, w io.Writer) error {
	components, err := database.GetComponentsByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("stream repomd: query components: %w", err)
	}

	ts := time.Now().Unix()
	revision := strconv.FormatInt(ts, 10)

	// Build minimal entries pointing to the streaming paths.
	// Checksums are left empty — this is the streaming/live path.
	entries := []RepomdData{
		{
			Type:         "primary",
			Location:     "repodata/primary.xml.gz",
			Checksum:     "",
			ChecksumType: "sha256",
			Timestamp:    ts,
			Size:         int64(len(components)) * 512, // rough estimate
		},
		{
			Type:         "filelists",
			Location:     "repodata/filelists.xml.gz",
			Checksum:     "",
			ChecksumType: "sha256",
			Timestamp:    ts,
		},
		{
			Type:         "other",
			Location:     "repodata/other.xml.gz",
			Checksum:     "",
			ChecksumType: "sha256",
			Timestamp:    ts,
		},
	}

	data, err := GenerateRepomdXML(revision, entries)
	if err != nil {
		return fmt.Errorf("stream repomd: generate: %w", err)
	}

	_, err = w.Write(data)

	return err
}

// StreamPrimaryXML writes a gzip-compressed primary.xml for the given
// repository directly to w, querying packages from the database.
func StreamPrimaryXML(ctx context.Context, database db.Database, repoID string, w io.Writer) error {
	components, err := database.GetComponentsByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("stream primary.xml: query: %w", err)
	}

	packages := componentsToPackages(components)

	xmlData, err := GeneratePrimaryXML(packages, "")
	if err != nil {
		return fmt.Errorf("stream primary.xml: generate: %w", err)
	}

	gz := gzip.NewWriter(w)
	if _, err := gz.Write(xmlData); err != nil {
		return fmt.Errorf("stream primary.xml: gzip write: %w", err)
	}

	return gz.Close()
}

// StreamFilelistsXML writes a gzip-compressed filelists.xml for the given
// repository directly to w.
func StreamFilelistsXML(ctx context.Context, database db.Database, repoID string, w io.Writer) error {
	components, err := database.GetComponentsByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("stream filelists.xml: query: %w", err)
	}

	packages := componentsToPackages(components)

	xmlData, err := GenerateFilelistsXML(packages)
	if err != nil {
		return fmt.Errorf("stream filelists.xml: generate: %w", err)
	}

	gz := gzip.NewWriter(w)
	if _, err := gz.Write(xmlData); err != nil {
		return fmt.Errorf("stream filelists.xml: gzip write: %w", err)
	}

	return gz.Close()
}

// StreamOtherXML writes a gzip-compressed other.xml for the given
// repository directly to w.
func StreamOtherXML(ctx context.Context, database db.Database, repoID string, w io.Writer) error {
	components, err := database.GetComponentsByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("stream other.xml: query: %w", err)
	}

	packages := componentsToPackages(components)

	xmlData, err := GenerateOtherXML(packages)
	if err != nil {
		return fmt.Errorf("stream other.xml: generate: %w", err)
	}

	gz := gzip.NewWriter(w)
	if _, err := gz.Write(xmlData); err != nil {
		return fmt.Errorf("stream other.xml: gzip write: %w", err)
	}

	return gz.Close()
}

// componentsToPackages converts database Component records to rpm.Package structs.
func componentsToPackages(components []models.Component) []pkgrpm.Package {
	packages := make([]pkgrpm.Package, 0, len(components))
	for _, c := range components {
		if c.Format != "rpm" {
			continue
		}

		packages = append(packages, pkgrpm.Package{
			Name:          c.Name,
			Version:       c.Version,
			Release:       c.Release,
			Epoch:         uint32(c.Epoch), //nolint:gosec // epoch fits in uint32
			Arch:          c.Arch,
			Summary:       c.Summary,
			Description:   c.Description,
			License:       c.License,
			URL:           c.URL,
			Vendor:        c.Vendor,
			Group:         c.Group,
			Packager:      c.Packager,
			SourceRPM:     c.SourceRPM,
			BuildHost:     c.BuildHost,
			BuildTime:     c.BuildTime,
			PackageSize:   c.PackageSize,
			InstalledSize: c.InstalledSize,
			ArchiveSize:   c.ArchiveSize,
			SHA256:        c.SHA256,
		})
	}

	return packages
}
