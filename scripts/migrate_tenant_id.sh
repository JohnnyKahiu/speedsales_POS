#!/usr/bin/env bash
# POS service — tenant_id rollout migration
# Usage: ./scripts/migrate_tenant_id.sh
# Run from the POS service root directory or anywhere; it finds its own path.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$SERVICE_DIR/.env"

# ── Colour helpers ─────────────────────────────────────────
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok()   { echo -e "${GREEN}[OK]${NC}  $*"; }
err()  { echo -e "${RED}[ERR]${NC} $*" >&2; exit 1; }
info() { echo -e "${YELLOW}[--]${NC}  $*"; }

# ── Load .env ──────────────────────────────────────────────
if [[ ! -f "$ENV_FILE" ]]; then
  err ".env not found at $ENV_FILE"
fi

# Strip quotes and spaces from values before exporting
set -o allexport
source <(grep -E '^(DB_HOST|DB_PORT|DB_NAME|DB_USER|DB_PASSWORD)\s*=' "$ENV_FILE" \
  | sed 's/[[:space:]]//g; s/="\(.*\)"/=\1/; s/"//g')
set +o allexport

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ipos}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"

export PGPASSWORD="$DB_PASSWORD"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  POS Service — tenant_id Migration"
echo "  DB : $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# ── Check psql is available ────────────────────────────────
if ! command -v psql &>/dev/null; then
  err "psql not found. Install postgresql-client and retry."
fi

# ── Check DB connection ────────────────────────────────────
info "Testing database connection..."
if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
     -c "SELECT 1" &>/dev/null; then
  err "Cannot connect to $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME — check credentials and that the DB exists."
fi
ok "Connected to $DB_NAME"
echo ""

# ── Run migration SQL ──────────────────────────────────────
info "Applying tenant_id migration..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
     --set ON_ERROR_STOP=1 \
     -f "$SCRIPT_DIR/tenant_id_migration.sql"

ok "Migration applied"
echo ""

# ── Verify ───────────────────────────────────────────────────
info "Verifying..."
MISSING=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "
  SELECT COUNT(*) FROM pg_tables t
  WHERE t.schemaname = 'public'
    AND NOT EXISTS (
      SELECT 1 FROM information_schema.columns c
      WHERE c.table_schema = t.schemaname
        AND c.table_name = t.tablename
        AND c.column_name = 'tenant_id'
    );
")

if [[ "$MISSING" -eq 0 ]]; then
  ok "tenant_id present on every table in public schema"
else
  err "$MISSING table(s) still missing tenant_id — check tenant_id_migration.sql for errors."
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "  ${GREEN}Migration complete.${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
