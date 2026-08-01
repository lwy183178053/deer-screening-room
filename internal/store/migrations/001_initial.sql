CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions(user_id);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);

CREATE TABLE wallets (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE site_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO site_settings(key,value) VALUES('redeem_notice','') ON CONFLICT(key) DO NOTHING;

CREATE TABLE wallet_entries (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delta BIGINT NOT NULL CHECK (delta <> 0),
    kind TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(kind, reference_id)
);
CREATE INDEX wallet_entries_user_idx ON wallet_entries(user_id, created_at DESC);

CREATE TABLE media_nodes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    base_url TEXT NOT NULL,
    online BOOLEAN NOT NULL DEFAULT FALSE,
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    available_bytes BIGINT NOT NULL DEFAULT 0 CHECK (available_bytes >= 0),
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE studios (
    id BIGSERIAL PRIMARY KEY,
    node_id BIGINT NOT NULL REFERENCES media_nodes(id),
    source_name TEXT NOT NULL,
    name TEXT NOT NULL,
    published BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(node_id, source_name)
);

CREATE TABLE videos (
    id BIGSERIAL PRIMARY KEY,
    node_id BIGINT NOT NULL REFERENCES media_nodes(id),
    studio_id BIGINT NOT NULL REFERENCES studios(id),
    media_key TEXT NOT NULL,
    source_title TEXT NOT NULL,
    title TEXT NOT NULL,
    poster_key TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    bit_rate BIGINT NOT NULL DEFAULT 0,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    video_codec TEXT NOT NULL DEFAULT '',
    audio_codec TEXT NOT NULL DEFAULT '',
    compatibility TEXT NOT NULL CHECK (compatibility IN ('ready','unsupported')),
    published BOOLEAN NOT NULL DEFAULT FALSE,
    available BOOLEAN NOT NULL DEFAULT TRUE,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(node_id, media_key)
);
CREATE INDEX videos_catalog_idx ON videos(published, available, updated_at DESC);
CREATE INDEX videos_studio_idx ON videos(studio_id, published, available);

CREATE TABLE video_entitlements (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(user_id, video_id)
);

CREATE TABLE payment_orders (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    credits BIGINT NOT NULL CHECK (credits > 0),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    payment_type TEXT NOT NULL CHECK (payment_type IN ('alipay','wxpay')),
    status TEXT NOT NULL CHECK (status IN ('pending','paid')),
    provider_trade_no TEXT UNIQUE,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX payment_orders_user_idx ON payment_orders(user_id, created_at DESC);

CREATE TABLE redeem_batches (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    credits BIGINT NOT NULL CHECK (credits > 0),
    code_count INTEGER NOT NULL CHECK (code_count > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE redeem_codes (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES redeem_batches(id),
    code_hash BYTEA NOT NULL UNIQUE,
    redeemed_by BIGINT REFERENCES users(id),
    redeemed_at TIMESTAMPTZ
);

CREATE TABLE playback_sessions (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX playback_sessions_active_idx ON playback_sessions(user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs(created_at DESC);
