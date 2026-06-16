#!/usr/bin/env bash
#
# Upstream Hub 一键部署脚本
#
#   首次部署：
#     git clone git@github.com:csbsgyl/upstream-hub.git
#     cd upstream-hub
#     ./scripts/deploy.sh
#
#   更新部署（已有目录里）：
#     ./scripts/deploy.sh
#
# 脚本会：
#   1. 检查 docker / docker compose 是否就绪
#   2. 首次运行时自动生成 .env（随机 APP_SECRET / POSTGRES_PASSWORD，默认开启登录）
#   3. git pull 拉取最新代码
#   4. docker compose up -d --build 重新构建并启动
#   5. 等待健康检查通过
#
# 默认账号：admin / admin —— 首次登录会强制要求修改密码。
#
set -euo pipefail

# ---- 切到仓库根目录（脚本在 scripts/ 下）----
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

# ---- 彩色日志 ----
c_info()  { printf '\033[36m[INFO]\033[0m  %s\n' "$*"; }
c_ok()    { printf '\033[32m[ OK ]\033[0m  %s\n' "$*"; }
c_warn()  { printf '\033[33m[WARN]\033[0m  %s\n' "$*"; }
c_err()   { printf '\033[31m[FAIL]\033[0m  %s\n' "$*" >&2; }

# ---- 1. 环境检查 ----
if ! command -v docker >/dev/null 2>&1; then
  c_err "未检测到 docker，请先安装：https://docs.docker.com/engine/install/"
  exit 1
fi

# docker compose v2（插件） 优先，回退到独立的 docker-compose
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  c_err "未检测到 docker compose，请安装 Compose 插件：https://docs.docker.com/compose/install/"
  exit 1
fi
c_ok "docker 与 compose 就绪（${COMPOSE}）"

# ---- 2. 生成 .env（仅首次）----
# 生成一个 URL 安全的随机串，优先 openssl，回退 /dev/urandom。
gen_secret() {
  local n="${1:-48}"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 "$n" | tr -d '\n/+=' | cut -c1-"$n"
  else
    LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c "$n"
  fi
}

if [[ -f .env ]]; then
  c_info ".env 已存在，跳过生成（如需重置请手动删除后重跑）"
else
  c_info "首次部署，生成 .env …"
  APP_SECRET_VAL="$(gen_secret 48)"
  PG_PASS_VAL="$(gen_secret 24)"
  if [[ -f .env.example ]]; then
    cp .env.example .env
    # 用生成值替换占位项；AUTH_ENABLED 默认开启
    # 注意用 | 作 sed 分隔符，避免随机串里的 / 干扰
    sed -i.bak \
      -e "s|^APP_SECRET=.*|APP_SECRET=${APP_SECRET_VAL}|" \
      -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PG_PASS_VAL}|" \
      -e "s|^AUTH_ENABLED=.*|AUTH_ENABLED=true|" \
      .env
    rm -f .env.bak
  else
    # .env.example 缺失时的兜底：写最小可用配置
    cat > .env <<EOF
POSTGRES_USER=upstreamhub
POSTGRES_PASSWORD=${PG_PASS_VAL}
POSTGRES_DB=upstreamhub
POSTGRES_PORT=54329
UPSTREAMHUB_HTTP_PORT=8080
UPSTREAMHUB_SERVER_MODE=release
UPSTREAMHUB_LOG_LEVEL=info
APP_SECRET=${APP_SECRET_VAL}
AUTH_ENABLED=true
EOF
  fi
  c_ok ".env 已生成（APP_SECRET / POSTGRES_PASSWORD 已随机化，AUTH_ENABLED=true）"
  c_warn "请妥善保存 .env —— APP_SECRET 一旦丢失，已加密的渠道凭据将无法解密！"
fi

# ---- 3. 拉取最新代码 ----
if [[ -d .git ]]; then
  c_info "拉取最新代码 …"
  CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)"
  if git pull --ff-only origin "${CURRENT_BRANCH}"; then
    c_ok "代码已更新到最新（${CURRENT_BRANCH}）"
  else
    c_warn "git pull 未能快进（可能有本地改动），用现有代码继续构建"
  fi
else
  c_warn "当前目录不是 git 仓库，跳过 git pull，用现有文件构建"
fi

# ---- 4. 构建并启动 ----
c_info "构建并启动容器（首次构建较慢，请耐心等待）…"
${COMPOSE} up -d --build

# ---- 5. 健康检查 ----
# 从 .env 读对外端口，默认 8080
HTTP_PORT="$(grep -E '^UPSTREAMHUB_HTTP_PORT=' .env 2>/dev/null | cut -d= -f2 || true)"
HTTP_PORT="${HTTP_PORT:-8080}"

c_info "等待服务健康检查（最多 90 秒）…"
HEALTHY=0
for i in $(seq 1 30); do
  if curl -fsS "http://localhost:${HTTP_PORT}/healthz" >/dev/null 2>&1; then
    HEALTHY=1
    break
  fi
  sleep 3
done

echo
if [[ "${HEALTHY}" == "1" ]]; then
  c_ok "部署完成 ✅"
  echo "  访问地址： http://localhost:${HTTP_PORT}"
  echo "  默认账号： admin / admin （首次登录会要求修改密码）"
else
  c_warn "健康检查超时，服务可能仍在启动。可手动排查："
  echo "  ${COMPOSE} ps"
  echo "  ${COMPOSE} logs -f app"
fi
