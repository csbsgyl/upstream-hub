#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

c_info() { printf '\033[36m[INFO]\033[0m  %s\n' "$*"; }
c_ok() { printf '\033[32m[ OK ]\033[0m  %s\n' "$*"; }
c_err() { printf '\033[31m[FAIL]\033[0m  %s\n' "$*" >&2; }

usage() {
  cat <<'EOF'
Usage:
  ./scripts/restore.sh ./backups/upstream-hub-YYYYmmdd-HHMMSS.sql.gz --yes

This restores SQL into the current Postgres database and can overwrite data.
Run ./scripts/backup.sh first unless you are restoring a fresh deployment.
EOF
}

SQL_FILE="${1:-}"
CONFIRM="${2:-}"
if [[ -z "${SQL_FILE}" || "${CONFIRM}" != "--yes" ]]; then
  usage
  exit 2
fi
if [[ ! -f "${SQL_FILE}" ]]; then
  c_err "备份文件不存在：${SQL_FILE}"
  exit 1
fi
if [[ "${SQL_FILE}" != *.sql.gz ]]; then
  c_err "只接受 .sql.gz 备份文件"
  exit 1
fi
if [[ ! -f .env ]]; then
  c_err "缺少 .env，无法确定数据库连接参数"
  exit 1
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  c_err "未检测到 docker compose"
  exit 1
fi

POSTGRES_USER="$(grep -E '^POSTGRES_USER=' .env | cut -d= -f2- || true)"
POSTGRES_DB="$(grep -E '^POSTGRES_DB=' .env | cut -d= -f2- || true)"
POSTGRES_USER="${POSTGRES_USER:-upstreamhub}"
POSTGRES_DB="${POSTGRES_DB:-upstreamhub}"

c_info "停止 app 容器，避免恢复期间写入"
${COMPOSE} stop app >/dev/null

c_info "清空 public schema"
${COMPOSE} exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1 \
  -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

c_info "导入 ${SQL_FILE}"
gunzip -c "${SQL_FILE}" | ${COMPOSE} exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1

c_info "启动 app 容器"
${COMPOSE} up -d app >/dev/null

c_ok "恢复完成"
