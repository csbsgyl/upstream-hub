#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

c_info() { printf '\033[36m[INFO]\033[0m  %s\n' "$*"; }
c_ok() { printf '\033[32m[ OK ]\033[0m  %s\n' "$*"; }
c_err() { printf '\033[31m[FAIL]\033[0m  %s\n' "$*" >&2; }

if ! command -v docker >/dev/null 2>&1; then
  c_err "未检测到 docker"
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

BACKUP_DIR="${UPSTREAMHUB_BACKUP_DIR:-./backups}"
mkdir -p "${BACKUP_DIR}"

if [[ ! -f .env ]]; then
  c_err "缺少 .env，无法确定数据库连接参数"
  exit 1
fi

POSTGRES_USER="$(grep -E '^POSTGRES_USER=' .env | cut -d= -f2- || true)"
POSTGRES_DB="$(grep -E '^POSTGRES_DB=' .env | cut -d= -f2- || true)"
POSTGRES_USER="${POSTGRES_USER:-upstreamhub}"
POSTGRES_DB="${POSTGRES_DB:-upstreamhub}"

STAMP="$(date +%Y%m%d-%H%M%S)"
SQL_FILE="${BACKUP_DIR}/upstream-hub-${STAMP}.sql.gz"
ENV_FILE="${BACKUP_DIR}/upstream-hub-${STAMP}.env"
META_FILE="${BACKUP_DIR}/latest.json"

c_info "导出数据库到 ${SQL_FILE}"
${COMPOSE} exec -T postgres pg_dump -U "${POSTGRES_USER}" "${POSTGRES_DB}" | gzip > "${SQL_FILE}"

c_info "备份 .env 到 ${ENV_FILE}"
cp .env "${ENV_FILE}"

cat > "${META_FILE}" <<EOF
{"created_at":"$(date -Iseconds)","sql":"$(basename "${SQL_FILE}")","env":"$(basename "${ENV_FILE}")"}
EOF

c_ok "备份完成"
printf '%s\n' "${SQL_FILE}"
