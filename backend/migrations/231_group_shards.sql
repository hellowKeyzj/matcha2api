-- GroupShard 为一个大分组下的用户/账号隔离分片；启用后调度只在用户所在 shard 的账号池内选择。
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS shard_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS shard_max_users_per_shard INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS shard_auto_create BOOLEAN NOT NULL DEFAULT TRUE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'groups_shard_max_users_per_shard_positive'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_shard_max_users_per_shard_positive
            CHECK (shard_max_users_per_shard > 0);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS group_shards (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS group_shards_group_id_name_key
    ON group_shards (group_id, name);
CREATE UNIQUE INDEX IF NOT EXISTS group_shards_id_group_id_key
    ON group_shards (id, group_id);
CREATE INDEX IF NOT EXISTS group_shards_group_id_idx
    ON group_shards (group_id);
CREATE INDEX IF NOT EXISTS group_shards_status_idx
    ON group_shards (status);

CREATE TABLE IF NOT EXISTS group_shard_users (
    group_shard_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_shard_id),
    CONSTRAINT group_shard_users_shard_group_fk
        FOREIGN KEY (group_shard_id, group_id)
        REFERENCES group_shards (id, group_id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS group_shard_users_group_id_user_id_key
    ON group_shard_users (group_id, user_id);
CREATE INDEX IF NOT EXISTS group_shard_users_group_id_idx
    ON group_shard_users (group_id);
CREATE INDEX IF NOT EXISTS group_shard_users_user_id_idx
    ON group_shard_users (user_id);

CREATE TABLE IF NOT EXISTS group_shard_accounts (
    group_shard_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, group_shard_id),
    CONSTRAINT group_shard_accounts_shard_group_fk
        FOREIGN KEY (group_shard_id, group_id)
        REFERENCES group_shards (id, group_id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS group_shard_accounts_group_id_account_id_key
    ON group_shard_accounts (group_id, account_id);
CREATE INDEX IF NOT EXISTS group_shard_accounts_group_id_idx
    ON group_shard_accounts (group_id);
CREATE INDEX IF NOT EXISTS group_shard_accounts_account_id_idx
    ON group_shard_accounts (account_id);
CREATE INDEX IF NOT EXISTS group_shard_accounts_group_platform_idx
    ON group_shard_accounts (group_id, platform);
CREATE INDEX IF NOT EXISTS group_shard_accounts_shard_platform_idx
    ON group_shard_accounts (group_shard_id, platform);
CREATE INDEX IF NOT EXISTS group_shard_accounts_priority_idx
    ON group_shard_accounts (priority);
