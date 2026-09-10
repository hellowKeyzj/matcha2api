CREATE TABLE IF NOT EXISTS matcha_packages (
    id BIGSERIAL PRIMARY KEY,
    package_key VARCHAR(160) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT matcha_packages_status_check
        CHECK (status IN ('draft', 'active', 'disabled', 'archived'))
);

CREATE TABLE IF NOT EXISTS matcha_package_versions (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES matcha_packages(id) ON DELETE CASCADE,
    version VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    runtime VARCHAR(64) NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL,
    object_key TEXT NOT NULL,
    owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    price_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    price_currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    surcharge_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    CONSTRAINT matcha_package_versions_kind_check CHECK (kind IN ('agent', 'skill')),
    CONSTRAINT matcha_package_versions_status_check CHECK (status IN ('draft', 'published', 'disabled', 'yanked')),
    CONSTRAINT matcha_package_versions_size_check CHECK (size_bytes > 0),
    CONSTRAINT matcha_package_versions_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT matcha_package_versions_price_check CHECK (price_amount >= 0),
    CONSTRAINT matcha_package_versions_unique UNIQUE (package_id, version)
);

CREATE TABLE IF NOT EXISTS matcha_package_entitlements (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES matcha_packages(id) ON DELETE CASCADE,
    version_id BIGINT REFERENCES matcha_package_versions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    source VARCHAR(64) NOT NULL DEFAULT 'manual',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version_key BIGINT GENERATED ALWAYS AS (COALESCE(version_id, 0)) STORED,
    CONSTRAINT matcha_package_entitlements_status_check CHECK (status IN ('active', 'revoked', 'expired'))
);

CREATE TABLE IF NOT EXISTS matcha_package_downloads (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES matcha_packages(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES matcha_package_versions(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    entitlement_id BIGINT REFERENCES matcha_package_entitlements(id) ON DELETE SET NULL,
    object_key TEXT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL,
    remote_addr TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    downloaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT matcha_package_downloads_size_check CHECK (size_bytes > 0),
    CONSTRAINT matcha_package_downloads_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS matcha_package_billing_policies (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES matcha_packages(id) ON DELETE CASCADE,
    version_id BIGINT REFERENCES matcha_package_versions(id) ON DELETE CASCADE,
    kind VARCHAR(32) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    unit VARCHAR(64) NOT NULL,
    policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT matcha_package_billing_policies_kind_check CHECK (kind IN ('price', 'surcharge')),
    CONSTRAINT matcha_package_billing_policies_amount_check CHECK (amount >= 0)
);

CREATE TABLE IF NOT EXISTS matcha_package_usage_records (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES matcha_packages(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES matcha_package_versions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entitlement_id BIGINT REFERENCES matcha_package_entitlements(id) ON DELETE SET NULL,
    request_id VARCHAR(160) NOT NULL,
    binding VARCHAR(512) NOT NULL,
    quantity DECIMAL(20,8) NOT NULL DEFAULT 0,
    unit VARCHAR(64) NOT NULL,
    amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT matcha_package_usage_records_quantity_check CHECK (quantity >= 0),
    CONSTRAINT matcha_package_usage_records_amount_check CHECK (amount >= 0),
    CONSTRAINT matcha_package_usage_records_dedupe UNIQUE (version_id, user_id, request_id, binding)
);

CREATE TABLE IF NOT EXISTS matcha_package_revenue_ledger (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES matcha_packages(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES matcha_package_versions(id) ON DELETE CASCADE,
    usage_record_id BIGINT REFERENCES matcha_package_usage_records(id) ON DELETE SET NULL,
    owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_type VARCHAR(32) NOT NULL,
    amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT matcha_package_revenue_ledger_entry_type_check CHECK (entry_type IN ('usage', 'payout')),
    CONSTRAINT matcha_package_revenue_ledger_status_check CHECK (status IN ('pending', 'posted', 'void')),
    CONSTRAINT matcha_package_revenue_ledger_amount_check CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_matcha_packages_owner_user_id ON matcha_packages(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_matcha_packages_status ON matcha_packages(status);
CREATE INDEX IF NOT EXISTS idx_matcha_package_versions_package_id ON matcha_package_versions(package_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_versions_kind_runtime ON matcha_package_versions(kind, runtime);
CREATE INDEX IF NOT EXISTS idx_matcha_package_versions_status ON matcha_package_versions(status);
CREATE INDEX IF NOT EXISTS idx_matcha_package_versions_sha256 ON matcha_package_versions(sha256);
CREATE UNIQUE INDEX IF NOT EXISTS idx_matcha_package_versions_object_key ON matcha_package_versions(object_key);
CREATE INDEX IF NOT EXISTS idx_matcha_package_entitlements_user_id ON matcha_package_entitlements(user_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_entitlements_package_user ON matcha_package_entitlements(package_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_matcha_package_entitlements_active_unique
    ON matcha_package_entitlements(package_id, user_id, version_key)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_matcha_package_downloads_version_id ON matcha_package_downloads(version_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_downloads_user_id ON matcha_package_downloads(user_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_downloads_downloaded_at ON matcha_package_downloads(downloaded_at);
CREATE INDEX IF NOT EXISTS idx_matcha_package_billing_policies_package_id ON matcha_package_billing_policies(package_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_billing_policies_active
    ON matcha_package_billing_policies(package_id, version_id, kind, unit)
    WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS idx_matcha_package_usage_records_user_id ON matcha_package_usage_records(user_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_usage_records_recorded_at ON matcha_package_usage_records(recorded_at);
CREATE INDEX IF NOT EXISTS idx_matcha_package_revenue_ledger_owner_user_id ON matcha_package_revenue_ledger(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_matcha_package_revenue_ledger_package_id ON matcha_package_revenue_ledger(package_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_matcha_package_revenue_ledger_usage_record_id ON matcha_package_revenue_ledger(usage_record_id) WHERE usage_record_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_matcha_package_revenue_ledger_status ON matcha_package_revenue_ledger(status);
