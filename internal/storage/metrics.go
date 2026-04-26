// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"io"
	"time"

	"github.com/openartifact/openartifact/internal/metrics"
)

type countingReader struct {
	r     io.Reader
	count int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	cr.count += int64(n)

	return n, err
}

type MetricsBlobStore struct {
	inner BlobStore
	reg   *metrics.Registry
}

func NewMetricsBlobStore(inner BlobStore, reg *metrics.Registry) *MetricsBlobStore {
	return &MetricsBlobStore{inner: inner, reg: reg}
}

func (m *MetricsBlobStore) Put(ctx context.Context, data io.Reader, metadata map[string]string) (string, error) {
	cr := &countingReader{r: data}

	id, err := m.inner.Put(ctx, cr, metadata)
	if err == nil {
		m.reg.BlobBytesTotal.WithLabelValues("put").Add(float64(cr.count))
	}

	return id, err
}

func (m *MetricsBlobStore) Get(ctx context.Context, id string) (io.ReadCloser, error) {
	rc, err := m.inner.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return &countingReadCloser{ReadCloser: rc, reg: m.reg}, nil
}

func (m *MetricsBlobStore) Delete(ctx context.Context, id string) error {
	return m.inner.Delete(ctx, id)
}

func (m *MetricsBlobStore) Exists(ctx context.Context, id string) (bool, error) {
	return m.inner.Exists(ctx, id)
}

func (m *MetricsBlobStore) GetMetadata(ctx context.Context, id string) (map[string]string, error) {
	return m.inner.GetMetadata(ctx, id)
}

func (m *MetricsBlobStore) List(ctx context.Context, prefix string, limit int, offset int) ([]string, error) {
	return m.inner.List(ctx, prefix, limit, offset)
}

func (m *MetricsBlobStore) GetSize(ctx context.Context, id string) (int64, error) {
	return m.inner.GetSize(ctx, id)
}

func (m *MetricsBlobStore) ExistsBySHA256(ctx context.Context, sha256 string) (string, bool, error) {
	return m.inner.ExistsBySHA256(ctx, sha256)
}

func (m *MetricsBlobStore) PresignURL(ctx context.Context, sha256 string, ttl time.Duration) (string, error) {
	return m.inner.PresignURL(ctx, sha256, ttl)
}

type countingReadCloser struct {
	io.ReadCloser

	reg   *metrics.Registry
	count int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)

	c.count += int64(n)
	if err == io.EOF {
		c.reg.BlobBytesTotal.WithLabelValues("get").Add(float64(c.count))
	}

	return n, err
}
