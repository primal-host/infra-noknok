package database

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token      TEXT NOT NULL UNIQUE,
    did        TEXT NOT NULL,
    handle     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions (token);

CREATE TABLE IF NOT EXISTS users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    did        TEXT NOT NULL UNIQUE,
    handle     TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS services (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL,
    icon_url    TEXT NOT NULL DEFAULT '',
    admin_role  TEXT NOT NULL DEFAULT 'admin',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE services ADD COLUMN IF NOT EXISTS admin_role TEXT NOT NULL DEFAULT 'admin';

CREATE TABLE IF NOT EXISTS grants (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'user',
    granted_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, service_id)
);
ALTER TABLE grants ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';

CREATE TABLE IF NOT EXISTS oauth_requests (
    state      TEXT PRIMARY KEY,
    data       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_sessions (
    did        TEXT NOT NULL,
    session_id TEXT NOT NULL,
    data       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (did, session_id)
);
`
