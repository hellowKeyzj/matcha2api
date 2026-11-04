-- Soft deletion does not trigger membership ON DELETE CASCADE.
DELETE FROM group_shard_users gsu
USING users u
WHERE u.id = gsu.user_id AND u.deleted_at IS NOT NULL;

-- Bindings cascade; provider accounts and parent-group membership remain intact.
DELETE FROM group_shards gs
WHERE NOT EXISTS (
    SELECT 1
    FROM group_shard_users gsu
    JOIN users u ON u.id = gsu.user_id AND u.deleted_at IS NULL
    WHERE gsu.group_shard_id = gs.id
);
