CREATE TABLE IF NOT EXISTS desktop_issues (
    id BIGSERIAL PRIMARY KEY,
    reporter_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reporter_email VARCHAR(255) NOT NULL DEFAULT '',
    reporter_username VARCHAR(128) NOT NULL DEFAULT '',
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    kind VARCHAR(32) NOT NULL DEFAULT 'bug',
    severity VARCHAR(32) NOT NULL DEFAULT 'major',
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    labels JSONB NOT NULL DEFAULT '[]'::jsonb,
    assignee_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    assignee_name VARCHAR(128) NOT NULL DEFAULT '',
    fix_version VARCHAR(64) NOT NULL DEFAULT '',
    app_version VARCHAR(64) NOT NULL DEFAULT '',
    client_os VARCHAR(128) NOT NULL DEFAULT '',
    client_arch VARCHAR(32) NOT NULL DEFAULT '',
    client_route VARCHAR(255) NOT NULL DEFAULT '',
    client_locale VARCHAR(32) NOT NULL DEFAULT '',
    client_meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    has_diagnostics BOOLEAN NOT NULL DEFAULT FALSE,
    image_count INTEGER NOT NULL DEFAULT 0,
    video_count INTEGER NOT NULL DEFAULT 0,
    comment_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    CONSTRAINT desktop_issues_kind_check CHECK (kind IN ('bug', 'feature', 'experience')),
    CONSTRAINT desktop_issues_severity_check CHECK (severity IN ('blocker', 'major', 'minor')),
    CONSTRAINT desktop_issues_status_check CHECK (status IN ('open', 'in_progress', 'resolved', 'closed'))
);

CREATE INDEX IF NOT EXISTS idx_desktop_issues_status_created
    ON desktop_issues (status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_desktop_issues_reporter_created
    ON desktop_issues (reporter_user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_desktop_issues_created
    ON desktop_issues (created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS desktop_issue_attachments (
    id BIGSERIAL PRIMARY KEY,
    issue_id BIGINT NOT NULL REFERENCES desktop_issues(id) ON DELETE CASCADE,
    category VARCHAR(32) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(128) NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    object_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT desktop_issue_attachments_category_check CHECK (category IN ('image', 'video', 'diagnostics')),
    CONSTRAINT desktop_issue_attachments_size_check CHECK (size_bytes > 0),
    CONSTRAINT desktop_issue_attachments_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$')
);

CREATE INDEX IF NOT EXISTS idx_desktop_issue_attachments_issue
    ON desktop_issue_attachments (issue_id, id ASC);

CREATE TABLE IF NOT EXISTS desktop_issue_events (
    id BIGSERIAL PRIMARY KEY,
    issue_id BIGINT NOT NULL REFERENCES desktop_issues(id) ON DELETE CASCADE,
    event_type VARCHAR(32) NOT NULL,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_name VARCHAR(255) NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT desktop_issue_events_type_check CHECK (
        event_type IN (
            'created',
            'note',
            'status_changed',
            'labels_changed',
            'assignee_changed',
            'severity_changed',
            'fix_version_changed'
        )
    )
);

CREATE INDEX IF NOT EXISTS idx_desktop_issue_events_issue
    ON desktop_issue_events (issue_id, created_at ASC, id ASC);
