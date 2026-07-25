-- ============================================================
-- Speed Sales — POS Service: tenant_id rollout
-- Adds a tenant_id column (+ index) to every base table in the
-- public schema. Safe to run multiple times (fully idempotent) —
-- ADD COLUMN/CREATE INDEX both use IF NOT EXISTS, and the loop
-- picks up tables dynamically so re-running after new tables are
-- added still fills in the gaps.
-- ============================================================

DO $$
DECLARE
    tbl text;
BEGIN
    FOR tbl IN
        SELECT tablename FROM pg_tables
        WHERE schemaname = 'public'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I ADD COLUMN IF NOT EXISTS tenant_id UUID NOT NULL DEFAULT ''00000000-0000-0000-0000-000000000000''',
            tbl
        );
        EXECUTE format(
            'CREATE INDEX IF NOT EXISTS %I ON %I (tenant_id)',
            'idx_' || tbl || '_tenant_id', tbl
        );
    END LOOP;
END;
$$;
