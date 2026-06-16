#!/usr/bin/env bash
#
# Upstream Hub 一条命令引导部署
#
# 新服务器上直接一行搞定（无需手动 git clone）：
#
#   curl -fsSL https://raw.githubusercontent.com/csbsgyl/upstream-hub/main/scripts/bootstrap.sh | bash
#
# 行为：
#   - 目标目录不存在  → git clone（https，公开仓库免认证）
#   - 目标目录已存在  → 进入并交给 deploy.sh 做 git pull 更新
#   - 然后执行 scripts/deploy.sh（生成 .env / 构建 / 启动 / 健康检查）
#
# 可用环境变量覆盖：
#   UPSTREAMHUB_DIR   克隆目录名，默认 upstream-hub
#
set -euo pipefail

REPO_URL="https://github.com/csbsgyl/upstream-hub.git"
TARGET_DIR="${UPSTREAMHUB_DIR:-upstream-hub}"

c_info() { printf '\033[36m[INFO]\033[0m  %s\n' "$*"; }
c_err()  { printf '\033[31m[FAIL]\033[0m  %s\n' "$*" >&2; }

if ! command -v git >/dev/null 2>&1; then
  c_err "未检测到 git，请先安装：sudo apt-get install -y git"
  exit 1
fi

if [[ -d "${TARGET_DIR}/.git" ]]; then
  c_info "已存在 ${TARGET_DIR}/，进入并更新部署 …"
  cd "${TARGET_DIR}"
else
  c_info "克隆仓库到 ${TARGET_DIR}/ …"
  git clone "${REPO_URL}" "${TARGET_DIR}"
  cd "${TARGET_DIR}"
fi

# 交给 deploy.sh：生成 .env / git pull / docker compose up --build / 健康检查
exec bash ./scripts/deploy.sh
