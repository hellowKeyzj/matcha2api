CREATE TABLE IF NOT EXISTS group_shared_accounts (
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, group_id)
);

CREATE INDEX IF NOT EXISTS group_shared_accounts_account_id_idx
    ON group_shared_accounts (account_id);
CREATE INDEX IF NOT EXISTS group_shared_accounts_group_platform_idx
    ON group_shared_accounts (group_id, platform);
