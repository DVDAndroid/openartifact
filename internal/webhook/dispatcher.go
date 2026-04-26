// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

// Event types.
const (
	EventArtifactUploaded   = "artifact.uploaded"
	EventArtifactDeleted    = "artifact.deleted"
	EventArtifactDownloaded = "artifact.downloaded"
	EventRepoCreated        = "repository.created"
	EventRepoDeleted        = "repository.deleted"
)

const (
	maxWebhookResponseBodySize     = 64 * 1024
	webhookResponseTruncatedMarker = "\n[response body truncated]"
)

type Dispatcher struct {
	db     database.Database
	log    *logger.Logger
	client *http.Client
	queue  chan *models.WebhookDelivery
	stopCh chan struct{}
	doneCh chan struct{}
}

func NewDispatcher(db database.Database, log *logger.Logger) *Dispatcher {
	return &Dispatcher{
		db:  db,
		log: log,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		queue:  make(chan *models.WebhookDelivery, 1000),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Dispatch queues a webhook event for all matching webhooks.
// repoID can be empty for global events.
func (d *Dispatcher) Dispatch(ctx context.Context, repoID, eventType string, payload map[string]any) {
	webhooks, err := d.db.ListWebhooksForEvent(ctx, repoID, eventType)
	if err != nil {
		d.log.Errorw("failed to list webhooks for event", "event", eventType, "error", err)
		return
	}

	for _, wh := range webhooks {
		delivery := &models.WebhookDelivery{
			WebhookID: wh.ID,
			EventType: eventType,
			Payload:   payload,
			Status:    "pending",
		}
		if err := d.db.CreateWebhookDelivery(ctx, delivery); err != nil {
			d.log.Errorw("failed to create webhook delivery", "webhook_id", wh.ID, "error", err)
			continue
		}

		select {
		case d.queue <- delivery:
		default:
			d.log.Warnw("webhook delivery queue full, dropping delivery", "webhook_id", wh.ID)
		}
	}
}

// Start begins processing the delivery queue and retry loop.
func (d *Dispatcher) Start() {
	// Queue processor goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.log.Errorw("panic in goroutine", "panic", r)
			}
		}()
		for {
			select {
			case delivery := <-d.queue:
				wh, err := d.db.GetWebhook(context.Background(), delivery.WebhookID)
				if err != nil {
					d.log.Errorw("failed to get webhook", "webhook_id", delivery.WebhookID, "error", err)
					continue
				}

				if wh == nil {
					d.log.Warnw("webhook not found", "webhook_id", delivery.WebhookID)
					continue
				}

				d.deliver(wh, delivery)
			case <-d.stopCh:
				return
			}
		}
	}()

	// Retry worker goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.log.Errorw("panic in goroutine", "panic", r)
			}
		}()
		defer close(d.doneCh)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				deliveries, err := d.db.ListPendingWebhookDeliveries(ctx)

				cancel()

				if err != nil {
					d.log.Errorw("failed to list pending webhook deliveries", "error", err)
					continue
				}

				for _, delivery := range deliveries {
					wh, err := d.db.GetWebhook(context.Background(), delivery.WebhookID)
					if err != nil {
						d.log.Errorw("failed to get webhook", "webhook_id", delivery.WebhookID, "error", err)
						continue
					}

					if wh == nil {
						d.log.Warnw("webhook not found", "webhook_id", delivery.WebhookID)
						continue
					}

					d.deliver(wh, delivery)
				}
			case <-d.stopCh:
				return
			}
		}
	}()
}

// Stop gracefully shuts down the dispatcher.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	<-d.doneCh
}

// deliver attempts to deliver a webhook.
func (d *Dispatcher) deliver(wh *models.Webhook, delivery *models.WebhookDelivery) {
	payloadJSON, err := json.Marshal(delivery.Payload)
	if err != nil {
		d.log.Errorw("failed to marshal payload", "webhook_id", wh.ID, "error", err)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, wh.URL, bytes.NewReader(payloadJSON))
	if err != nil {
		d.log.Errorw("failed to create request", "webhook_id", wh.ID, "error", err)
		d.markFailed(delivery, wh)

		return
	}

	req.Header.Set("Content-Type", wh.ContentType)
	req.Header.Set("User-Agent", "OpenArtifact-Webhook/1.0")

	// Add HMAC signature if secret is provided
	if wh.SecretEnc != "" {
		signature := computeHMAC(payloadJSON, []byte(wh.SecretEnc))
		req.Header.Set("X-Hub-Signature-256", "sha256="+signature)
	}

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wh.TimeoutSeconds)*time.Second)
	defer cancel()

	req = req.WithContext(ctx)

	// Make the request
	resp, err := d.client.Do(req)
	if err != nil {
		d.log.Errorw("failed to send webhook", "webhook_id", wh.ID, "error", err)
		d.markFailed(delivery, wh)

		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxWebhookResponseBodySize+1))
	if err != nil {
		d.log.Errorw("failed to read response body", "webhook_id", wh.ID, "error", err)
	}
	if len(respBody) > maxWebhookResponseBodySize {
		respBody = append(respBody[:maxWebhookResponseBodySize-len(webhookResponseTruncatedMarker)], webhookResponseTruncatedMarker...)
	}

	delivery.ResponseStatus = resp.StatusCode
	delivery.ResponseBody = string(respBody)
	delivery.AttemptCount++

	// Check if successful
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		delivery.Status = "success"
		now := time.Now()

		delivery.DeliveredAt = &now
		if err := d.db.UpdateWebhookDelivery(context.Background(), delivery); err != nil {
			d.log.Errorw("failed to update webhook delivery", "webhook_id", wh.ID, "error", err)
		}

		d.log.Infow("webhook delivered successfully", "webhook_id", wh.ID, "status", resp.StatusCode)

		return
	}

	// Failed - check if we should retry
	d.markFailed(delivery, wh)
}

// markFailed marks a delivery as failed and schedules retry if applicable.
func (d *Dispatcher) markFailed(delivery *models.WebhookDelivery, wh *models.Webhook) {
	if delivery.AttemptCount < wh.MaxRetries {
		// Schedule retry with exponential backoff: 2^attempt * 30s
		backoffSeconds := math.Pow(2, float64(delivery.AttemptCount)) * 30
		nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)
		delivery.NextRetryAt = &nextRetry
		delivery.Status = "pending"
	} else {
		// Max retries exceeded
		delivery.Status = "failed"
	}

	if err := d.db.UpdateWebhookDelivery(context.Background(), delivery); err != nil {
		d.log.Errorw("failed to update webhook delivery", "webhook_id", wh.ID, "error", err)
	}

	if delivery.Status == "failed" {
		d.log.Warnw("webhook delivery failed permanently", "webhook_id", wh.ID, "attempts", delivery.AttemptCount)
	} else {
		d.log.Infow("webhook delivery scheduled for retry", "webhook_id", wh.ID, "attempts", delivery.AttemptCount)
	}
}

// computeHMAC computes HMAC-SHA256 signature.
func computeHMAC(data, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(data)

	return hex.EncodeToString(h.Sum(nil))
}
