-- SPDX-FileCopyrightText: The OpenArtifact Authors
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Android, part 2: validate the widened format CHECK outside the migration
-- transaction. Re-runnable (validating a valid constraint is a no-op).

ALTER TABLE repositories VALIDATE CONSTRAINT chk_repo_format;
