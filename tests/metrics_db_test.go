// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openartifact/openartifact/internal/database/models"
)

// detailsJSON creates a DetailsJSON field with message.
func detailsJSON2(message string) json.RawMessage {
	m := map[string]string{"message": message}
	b, _ := json.Marshal(m)
	return b
}

func TestMockDatabase_InsertAuditLog_RecordsEvent(t *testing.T) {
	db := newMockDatabase()
	ctx := context.Background()

	ev := &models.AuditLog{
		Action:      "test.action",
		ActorID:     new("alice"),
		DetailsJSON: detailsJSON2("success"),
	}

	if err := db.InsertAuditLog(ctx, ev); err != nil {
		t.Fatalf("InsertAuditLog: %v", err)
	}

	if len(db.AuditLogs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(db.AuditLogs))
	}

	if db.AuditLogs[0].Action != "test.action" {
		t.Errorf("expected action test.action, got %s", db.AuditLogs[0].Action)
	}

	if db.AuditLogs[0].ActorID == nil || *db.AuditLogs[0].ActorID != "alice" {
		actual := ""
		if db.AuditLogs[0].ActorID != nil {
			actual = *db.AuditLogs[0].ActorID
		}
		t.Errorf("expected actor alice, got %s", actual)
	}
}

func TestMockDatabase_InsertAuditLog_MultipleEvents(t *testing.T) {
	db := newMockDatabase()
	ctx := context.Background()

	for i := range 3 {
		if err := db.InsertAuditLog(ctx, &models.AuditLog{Action: "ev", ActorID: new("bot")}); err != nil {
			t.Fatalf("InsertAuditLog[%d]: %v", i, err)
		}
	}

	if len(db.AuditLogs) != 3 {
		t.Errorf("expected 3 audit logs, got %d", len(db.AuditLogs))
	}
}
