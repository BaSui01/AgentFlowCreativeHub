-- Multi-tenant core schema migration

CREATE TABLE IF NOT EXISTS tenants (
    id              UUID PRIMARY KEY,
    name            VARCHAR(255) NOT NULL UNIQUE,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending_verification',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    username        VARCHAR(255) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending_activation',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_users_tenant_email UNIQUE (tenant_id, email),
    CONSTRAINT uq_users_tenant_username UNIQUE (tenant_id, username)
);

CREATE TABLE IF NOT EXISTS roles (
    id              UUID PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_roles_tenant_name UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS permissions (
    id              UUID PRIMARY KEY,
    tenant_id       UUID        NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource        VARCHAR(255) NOT NULL,
    action          VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS user_roles (
    id              UUID PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id         UUID        NOT NULL REFERENCES roles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS role_permissions (
    id              UUID PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id         UUID        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id   UUID        NOT NULL REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tenant_configs (
    tenant_id       UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT,
    logo_url        TEXT,
    language        VARCHAR(10)  NOT NULL DEFAULT 'zh-CN',
    timezone        VARCHAR(64)  NOT NULL DEFAULT 'Asia/Shanghai',
    feature_flags   JSONB        NOT NULL DEFAULT '{}'::JSONB
);

CREATE TABLE IF NOT EXISTS tenant_quotas (
    tenant_id       UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    max_users       INTEGER      NOT NULL DEFAULT 100,
    max_storage_mb  INTEGER      NOT NULL DEFAULT 10240,
    max_workflows   INTEGER      NOT NULL DEFAULT 100,
    used_users      INTEGER      NOT NULL DEFAULT 0,
    used_storage_mb INTEGER      NOT NULL DEFAULT 0,
    used_workflows  INTEGER      NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id              UUID PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id         UUID        NULL REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(255) NOT NULL,
    resource        VARCHAR(255) NOT NULL,
    details         JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_tenant ON users (tenant_id);
CREATE INDEX IF NOT EXISTS idx_roles_tenant ON roles (tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_tenant_user ON user_roles (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_tenant_role ON role_permissions (tenant_id, role_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_created_at ON audit_logs (tenant_id, created_at DESC);
