-- Ensure exactly one non-deleted group can be the default shard supply group.
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY id) AS rn
    FROM groups
    WHERE shard_enabled = TRUE AND deleted_at IS NULL
)
UPDATE groups
SET shard_enabled = FALSE,
    shard_auto_create = TRUE
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

UPDATE groups
SET shard_auto_create = TRUE
WHERE shard_enabled = TRUE AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_single_default_shard_group
ON groups (shard_enabled)
WHERE shard_enabled = TRUE AND deleted_at IS NULL;
