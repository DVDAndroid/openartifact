-- name: SetArtifactProperty :exec
WITH ref AS (
    INSERT INTO artifact_refs (repository_id, artifact_path, format)
    VALUES ($1, $2, COALESCE((SELECT format FROM repositories WHERE id = $1), 'unknown'))
    ON CONFLICT (repository_id, artifact_path) DO UPDATE SET artifact_path = EXCLUDED.artifact_path
    RETURNING id
), upd AS (
    UPDATE artifact_properties ap
    SET value = $4, updated_at = now()
    FROM ref
    WHERE ap.repository_id = $1 AND ap.artifact_ref_id = ref.id AND ap.key = $3
    RETURNING ap.id
)
INSERT INTO artifact_properties (repository_id, artifact_ref_id, key, value)
SELECT $1, ref.id, $3, $4 FROM ref
WHERE NOT EXISTS (SELECT 1 FROM upd);

-- name: GetArtifactProperty :one
SELECT ap.value FROM artifact_properties ap
JOIN artifact_refs ar ON ar.id = ap.artifact_ref_id
WHERE ap.repository_id = $1 AND ar.artifact_path = $2 AND ap.key = $3;

-- name: DeleteArtifactProperty :exec
DELETE FROM artifact_properties ap
USING artifact_refs ar
WHERE ap.artifact_ref_id = ar.id AND ap.repository_id = $1 AND ar.artifact_path = $2 AND ap.key = $3;

-- name: ListArtifactProperties :many
SELECT ap.key, ap.value FROM artifact_properties ap
JOIN artifact_refs ar ON ar.id = ap.artifact_ref_id
WHERE ap.repository_id = $1 AND ar.artifact_path = $2 ORDER BY ap.key;

-- name: SearchByProperty :many
SELECT DISTINCT ar.artifact_path AS artifact_id FROM artifact_properties ap
JOIN artifact_refs ar ON ar.id = ap.artifact_ref_id
WHERE ap.repository_id = $1 AND ap.key = $2 AND ap.value = $3 ORDER BY ar.artifact_path;
