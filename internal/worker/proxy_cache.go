// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"context"
	"time"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/pkg/logger"
)

type ProxyCacheWorker struct {
	db       database.Database
	log      *logger.Logger
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func NewProxyCacheWorker(db database.Database, log *logger.Logger) *ProxyCacheWorker {
	return &ProxyCacheWorker{
		db:       db,
		log:      log,
		interval: 5 * time.Minute,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (w *ProxyCacheWorker) Start() {
	go func() {
		defer close(w.doneCh)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				n, err := w.db.DeleteExpiredProxyCacheEntries(ctx)

				cancel()

				if err != nil {
					w.log.Errorw("proxy cache eviction failed", "error", err)
				} else if n > 0 {
					w.log.Infow("proxy cache eviction complete", "evicted", n)
				}
			case <-w.stopCh:
				return
			}
		}
	}()
}

func (w *ProxyCacheWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh
}
