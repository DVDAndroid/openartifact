// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package audit

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/pkg/logger"
)

//nolint:gochecknoglobals // process-lifetime singleton logger
var defaultLogger atomic.Pointer[Logger]

func SetDefault(l *Logger) {
	defaultLogger.Store(l)
}

func Default() *Logger {
	return defaultLogger.Load()
}

// AuditEvent is the canonical event type sent to the audit logger.
// Fields correspond to the audit_logs schema.
type AuditEvent struct {
	Action       string  // required; validated action name (e.g. auth.token_validate)
	ActorType    string  // "user", "system", "token", "sso"
	ActorID      *string // UUID of the user/token
	Actor        string  // human-readable identifier (username, token name) — stored in details_json
	RepositoryID *string
	ResourceType *string
	ResourceID   *string
	OrgID        *string
	IPAddress    *string
	Details      string // free-text; stored in details_json as {"message": "..."}
}

type Logger struct {
	ch   chan AuditEvent
	db   database.Database
	log  *logger.Logger
	reg  *metrics.Registry
	done chan struct{}
}

func NewLogger(db database.Database, log *logger.Logger, reg *metrics.Registry, bufferSize int) *Logger {
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	l := &Logger{
		ch:   make(chan AuditEvent, bufferSize),
		db:   db,
		log:  log,
		reg:  reg,
		done: make(chan struct{}),
	}
	go l.run()

	return l
}

func nilActorType(t string) *string {
	if t == "" {
		return nil
	}
	return &t
}

func (l *Logger) run() {
	defer func() {
		if r := recover(); r != nil {
			l.log.Errorw("audit worker panic", "error", r)
			go l.run()
		}
	}()

	for event := range l.ch {
		detailsJSON := "{}"
		if event.Details != "" {
			detailsJSON = buildDetailsJSON(event.Details, event.Actor)
		} else if event.Actor != "" {
			detailsJSON = buildDetailsJSON("", event.Actor)
		}

		entry := &models.AuditLog{
			Action:       event.Action,
			ActorID:      event.ActorID,
			ActorType:    nilActorType(event.ActorType),
			DetailsJSON:  json.RawMessage(detailsJSON),
			ResourceType: event.ResourceType,
			ResourceID:   event.ResourceID,
			RepositoryID: event.RepositoryID,
			OrgID:        event.OrgID,
			IPAddress:    event.IPAddress,
		}
		if err := l.db.InsertAuditLog(context.Background(), entry); err != nil {
			l.log.Errorw("audit log insert failed", "error", err)
		}
	}

	close(l.done)
}

func buildDetailsJSON(message, actor string) string {
	// Minimal JSON builder without importing encoding/json for speed.
	if message != "" && actor != "" {
		return `{"message":"` + escapeJSON(message) + `","actor":"` + escapeJSON(actor) + `"}`
	}
	if message != "" {
		return `{"message":"` + escapeJSON(message) + `"}`
	}
	return `{"actor":"` + escapeJSON(actor) + `"}`
}

func escapeJSON(s string) string {
	// Replace JSON meta-characters in a fire-and-forget audit logger.
	b := make([]byte, 0, len(s)+8)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, s[i])
		}
	}
	return string(b)
}

func (l *Logger) Log(_ context.Context, event AuditEvent) {
	select {
	case l.ch <- event:
	default:
		l.reg.AuditDroppedTotal.Inc()
	}
}

func (l *Logger) Shutdown(ctx context.Context) error {
	close(l.ch)

	select {
	case <-l.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
