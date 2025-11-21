-- RAG & Tool System schema migration

CREATE TABLE IF NOT EXISTS knowledge_bases (
	id                     UUID PRIMARY KEY,
	tenant_id              UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
	name                   VARCHAR(255) NOT NULL,
	description            TEXT,
	visibility_scope       VARCHAR(64)  NOT NULL DEFAULT 'tenant',
	default_embedding_model VARCHAR(255),
	created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
	updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_kb_tenant_name UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS knowledge_documents (
	id                  UUID PRIMARY KEY,
	knowledge_base_id   UUID        NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
	source_type         VARCHAR(64) NOT NULL,
	source_uri          TEXT        NOT NULL,
	version             VARCHAR(255),
	status              VARCHAR(32) NOT NULL DEFAULT 'pending_index',
	created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
	id            UUID PRIMARY KEY,
	document_id   UUID        NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
	chunk_index   INTEGER     NOT NULL,
	content       TEXT        NOT NULL,
	metadata      JSONB       NOT NULL DEFAULT '{}'::JSONB
);

CREATE TABLE IF NOT EXISTS rag_query_logs (
	id                   UUID PRIMARY KEY,
	tenant_id            UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
	user_id              UUID        NULL REFERENCES users(id) ON DELETE SET NULL,
	knowledge_base_ids   UUID[]      NOT NULL,
	top_k                INTEGER     NOT NULL,
	score_threshold      DOUBLE PRECISION,
	retrieved_count      INTEGER     NOT NULL,
	avg_score            DOUBLE PRECISION,
	latency_ms           INTEGER     NOT NULL,
	trace_id             VARCHAR(255),
	session_id           VARCHAR(255),
	created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tools (
	id                UUID PRIMARY KEY,
	tenant_id         UUID        NULL REFERENCES tenants(id) ON DELETE CASCADE,
	name              VARCHAR(255) NOT NULL,
	category          VARCHAR(64)  NOT NULL,
	description       TEXT,
	input_schema      JSONB        NOT NULL,
	output_schema     JSONB        NOT NULL,
	sensitivity_level VARCHAR(32)  NOT NULL DEFAULT 'normal',
	status            VARCHAR(32)  NOT NULL DEFAULT 'active',
	created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
	updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_tools_tenant_name UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS tool_versions (
	id         UUID PRIMARY KEY,
	tool_id    UUID        NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
	version    VARCHAR(64) NOT NULL,
	impl_type  VARCHAR(32) NOT NULL,
	impl_ref   TEXT        NOT NULL,
	config     JSONB       NOT NULL DEFAULT '{}'::JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_tool_versions_tool_version UNIQUE (tool_id, version)
);

CREATE TABLE IF NOT EXISTS tool_call_logs (
	id               UUID PRIMARY KEY,
	tenant_id        UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
	user_id          UUID        NULL REFERENCES users(id) ON DELETE SET NULL,
	tool_id          UUID        NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
	tool_version_id  UUID        NOT NULL REFERENCES tool_versions(id) ON DELETE CASCADE,
	status           VARCHAR(32) NOT NULL,
	latency_ms       INTEGER     NOT NULL,
	error_code       VARCHAR(64),
	error_message    TEXT,
	trace_id         VARCHAR(255),
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kb_tenant ON knowledge_bases (tenant_id);
CREATE INDEX IF NOT EXISTS idx_kd_kb ON knowledge_documents (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_kc_document ON knowledge_chunks (document_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_rag_logs_tenant_created_at ON rag_query_logs (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tools_tenant ON tools (tenant_id);
CREATE INDEX IF NOT EXISTS idx_tool_call_logs_tenant_created_at ON tool_call_logs (tenant_id, created_at DESC);
