-- 0011_rbac_permissions.sql
-- 扩展权限表结构，引入 code/name/category 以及更新时间戳，方便自定义角色与权限编排

BEGIN;

ALTER TABLE permissions
    ADD COLUMN IF NOT EXISTS code VARCHAR(128),
    ADD COLUMN IF NOT EXISTS name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS category VARCHAR(64),
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

UPDATE permissions
SET code = COALESCE(code, LOWER(resource || ':' || action)),
    name = COALESCE(name, INITCAP(resource) || ' ' || INITCAP(action)),
    category = COALESCE(category, resource),
    updated_at = NOW()
WHERE code IS NULL OR name IS NULL OR category IS NULL;

ALTER TABLE permissions
    ALTER COLUMN code SET NOT NULL,
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_code ON permissions(code);

CREATE OR REPLACE FUNCTION set_permissions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_permissions_updated_at ON permissions;
CREATE TRIGGER trg_permissions_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION set_permissions_updated_at();

COMMIT;
