-- SPDX-FileCopyrightText: The OpenArtifact Authors
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Checks the audit_logs rows written before 025 against the widened action list
-- without blocking inserts (SHARE UPDATE EXCLUSIVE). A no-op once validated.
ALTER TABLE audit_logs VALIDATE CONSTRAINT chk_audit_action;
