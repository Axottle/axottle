-- Common account, project, settings and audit schema. No engine tables.
CREATE TABLE IF NOT EXISTS system_settings (
    key         TEXT PRIMARY KEY,
    value_json  JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS setup_state (
    key         TEXT PRIMARY KEY,
    completed   BOOLEAN NOT NULL DEFAULT FALSE,
    data_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admins (
    id              TEXT PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'admin',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    admin_id    TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires    ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS audit_log (
    id              BIGSERIAL PRIMARY KEY,
    actor_type      TEXT NOT NULL,
    actor_id        TEXT NOT NULL,
    action          TEXT NOT NULL,
    resource_type   TEXT,
    resource_id     TEXT,
    metadata_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor   ON audit_log(actor_type, actor_id);


CREATE TABLE IF NOT EXISTS secrets (
    id               text PRIMARY KEY,
    -- Free-form discriminator. Examples: 'cdn_yandex_service_account',
    -- 'cdn_timeweb_api_key', 'cdn_manual_notes'. Keep it short.
    kind             text NOT NULL,
    -- AES-GCM ciphertext + 12-byte nonce stored side-by-side. We keep them
    -- as separate columns so future key-rotation tooling can re-encrypt
    -- without touching nonces.
    value_encrypted  bytea NOT NULL,
    nonce            bytea NOT NULL,
    -- Optional metadata for the secret itself (NEVER store secret material
    -- here). Free-form; e.g. `{"hint":"sa-key.json","filename":"..."}`.
    metadata_json    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT NOW(),
    updated_at       timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_secrets_kind ON secrets(kind);

-- TOTP configuration for panel administrators. The base32 secret itself is
-- stored in the existing AES-256-GCM secretstore; this table only links it to
-- an administrator and tracks confirmation/replay state.
CREATE TABLE IF NOT EXISTS admin_totp (
    admin_id           TEXT PRIMARY KEY REFERENCES admins(id) ON DELETE CASCADE,
    secret_id          TEXT NOT NULL UNIQUE REFERENCES secrets(id),
    confirmed_at       TIMESTAMPTZ,
    last_used_counter  BIGINT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Remove the encrypted payload when an admin (or their TOTP row) is removed.
CREATE OR REPLACE FUNCTION cleanup_admin_totp_secret() RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM secrets WHERE id = OLD.secret_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_cleanup_admin_totp_secret ON admin_totp;
CREATE TRIGGER trg_cleanup_admin_totp_secret
AFTER DELETE ON admin_totp
FOR EACH ROW EXECUTE FUNCTION cleanup_admin_totp_secret();
ALTER TABLE admins ADD COLUMN IF NOT EXISTS permissions JSONB NOT NULL DEFAULT '["*"]'::jsonb;
ALTER TABLE admins ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE admins ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('invited', 'active', 'disabled'));
ALTER TABLE admins ADD COLUMN IF NOT EXISTS is_bootstrap_superuser BOOLEAN NOT NULL DEFAULT FALSE;

-- A user may authenticate through Telegram alone, so a password is now optional.
-- The Go layer enforces "at least one login method" (password or a linked
-- identity); the column itself just stops being mandatory.
ALTER TABLE admins ALTER COLUMN password_hash DROP NOT NULL;

-- At most one bootstrap superuser can ever exist. A partial unique index over
-- the TRUE value (NULLs/FALSE are unconstrained) enforces it in the database,
-- so a second superuser cannot be created by any path.
CREATE UNIQUE INDEX IF NOT EXISTS idx_admins_single_superuser
    ON admins (is_bootstrap_superuser) WHERE is_bootstrap_superuser = TRUE;

-- ─── 2. External login identities (§2.2) ───────────────────────────────────
--
-- One row per (user, external provider). Telegram is the only provider today:
--   provider          = 'telegram'
--   provider_user_id  = the numeric Telegram User ID (Bot API / OIDC claim `id`)
--   subject           = the OIDC `sub` claim, filled on the first verified
--                       OIDC login (NULL until then — a pre-created/migrated
--                       identity is matched by provider_user_id until `sub`
--                       is learned, then upgraded).
CREATE TABLE IF NOT EXISTS auth_identities (
    id                  TEXT PRIMARY KEY,
    admin_id            TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL,
    subject             TEXT,
    provider_user_id    TEXT NOT NULL,
    username            TEXT NOT NULL DEFAULT '',
    first_name          TEXT NOT NULL DEFAULT '',
    last_name           TEXT NOT NULL DEFAULT '',
    picture_url         TEXT NOT NULL DEFAULT '',
    verified_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One external account maps to exactly one Axottle user.
    UNIQUE (provider, provider_user_id)
);

-- A given `sub` is likewise unique per provider, but only once it is known.
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_identities_subject
    ON auth_identities (provider, subject) WHERE subject IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_auth_identities_admin ON auth_identities (admin_id);

CREATE OR REPLACE FUNCTION protect_bootstrap_superuser() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.is_bootstrap_superuser THEN
            RAISE EXCEPTION 'bootstrap superuser cannot be deleted'
                USING ERRCODE = 'P0001';
        END IF;
        RETURN OLD;
    END IF;

    -- UPDATE: only constrain a row that IS the superuser.
    IF OLD.is_bootstrap_superuser THEN
        IF NEW.is_bootstrap_superuser IS DISTINCT FROM TRUE THEN
            RAISE EXCEPTION 'bootstrap superuser flag cannot be removed'
                USING ERRCODE = 'P0001';
        END IF;
        IF NEW.status <> 'active' THEN
            RAISE EXCEPTION 'bootstrap superuser cannot be disabled'
                USING ERRCODE = 'P0001';
        END IF;
        IF NOT (NEW.permissions @> '["*"]'::jsonb) THEN
            RAISE EXCEPTION 'bootstrap superuser must keep full access'
                USING ERRCODE = 'P0001';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_protect_bootstrap_superuser ON admins;
CREATE TRIGGER trg_protect_bootstrap_superuser
BEFORE UPDATE OR DELETE ON admins
FOR EACH ROW EXECUTE FUNCTION protect_bootstrap_superuser();
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_projects (
    admin_id TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    PRIMARY KEY (admin_id, project_id)
);
CREATE INDEX IF NOT EXISTS admin_projects_project_idx ON admin_projects(project_id, admin_id);

ALTER TABLE admins ADD COLUMN IF NOT EXISTS global_projects BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS category      TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS summary       TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS resource_name TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS before_json   JSONB;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS after_json    JSONB;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS changes_json  JSONB;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS status        TEXT NOT NULL DEFAULT 'ok';
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS method        TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS path          TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS request_id    TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS ip            TEXT;
CREATE INDEX IF NOT EXISTS idx_audit_log_category ON audit_log(category, created_at DESC);
