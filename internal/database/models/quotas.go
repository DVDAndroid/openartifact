// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type StorageQuota struct {
	ID                   string
	RepositoryID         string
	MaxSizeBytes         int64
	MaxArtifactCount     int
	WarnThresholdPercent int
	CurrentSizeBytes     int64
	CurrentArtifactCount int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (q *StorageQuota) IsExceeded() bool {
	if q.MaxSizeBytes > 0 && q.CurrentSizeBytes >= q.MaxSizeBytes {
		return true
	}

	if q.MaxArtifactCount > 0 && q.CurrentArtifactCount >= q.MaxArtifactCount {
		return true
	}

	return false
}

func (q *StorageQuota) UsagePercent() float64 {
	if q.MaxSizeBytes == 0 {
		return 0
	}

	return float64(q.CurrentSizeBytes) / float64(q.MaxSizeBytes) * 100
}
