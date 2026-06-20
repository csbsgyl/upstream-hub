# Upstream Hub

Upstream Hub 是面向 NewAPI / Sub2API 站点的上游渠道监控面板，用来集中管理多个上游，持续查看余额、模型倍率、倍率变化记录、通知推送和运维状态。

> 本项目基于 [worryzyy/upstream-hub](https://github.com/worryzyy/upstream-hub) 二次开发，感谢原作者 [@worryzyy](https://github.com/worryzyy) 的开源工作。本仓库聚焦更完整的部署、通知、运维和管理体验，适合自部署使用。

## 预览

![Upstream Hub 预览 1](docs/images/demo1.png)

![Upstream Hub 预览 2](docs/images/demo2.png)

![Upstream Hub 预览 3](docs/images/demo3.png)

![Upstream Hub 预览 4](docs/images/demo4.png)

## 功能亮点

- 多上游渠道管理，支持 NewAPI / Sub2API。
- 余额汇总、低余额告警、健康状态和最近采集状态展示。
- 模型倍率监控、倍率变化日志、倍率变动通知。
- 默认每 3 分钟同一轮同步余额和倍率，减少重复登录和重复通知。
- 余额趋势图支持 24 小时、7 天、30 天筛选；24 小时按 3 分钟采样桶聚合。
- 通知渠道支持 Telegram、Webhook、Email、企业微信、钉钉、飞书、Server酱。
- 通知渠道可按上游和分组订阅，支持倍率变化合并、涨跌方向过滤、静默分组和冷却时间。
- Cloudflare Turnstile 打码配置管理。
- 运维中心支持手动同步、立即备份、备份下载、失败通知重发、诊断包、日志清理。
- 网页一键更新，支持实时更新进度、阶段状态和日志尾部展示。
- 默认开启登录，默认账号 `admin` / `admin`，首次登录强制改密。
- 渠道原站点地址默认隐藏，鼠标悬停或键盘聚焦时才展开，减少截图泄露风险。

## 一键部署

新服务器上推荐直接执行：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/upstream-hub/main/scripts/bootstrap.sh | bash
```

脚本会自动完成：clone 仓库、检查 Docker / Compose、首次生成 `.env`、随机生成 `APP_SECRET` 和 `POSTGRES_PASSWORD`、按间隔执行部署前数据库备份、构建镜像、启动服务、等待健康检查。

> 前提：服务器已安装 `git`、`docker`、`docker compose`、`curl`。

启动后访问：

```text
http://localhost:8080
```

默认账号：

```text
admin / admin
```

首次登录会强制修改密码。

## 更新部署

如果已经 clone 过仓库，进入项目目录执行：

```bash
cd upstream-hub
./scripts/deploy.sh
```

部署脚本会执行 `git pull --ff-only`，然后重新构建并启动容器。已有数据库容器时，脚本默认 7 天自动备份一次数据库，避免每次更新都堆积备份文件。

需要手动备份时，可以在「运维中心」点“立即备份”，也可以运行：

```bash
./scripts/backup.sh
```

## 手动 Docker Compose

```bash
cp .env.example .env
```

至少需要设置：

```env
APP_SECRET=请替换为 32 字节以上随机字符串
POSTGRES_PASSWORD=请替换为数据库密码
```

启动：

```bash
docker compose up -d --build
```

> 本仓库是二开版，`docker-compose.yml` 默认从当前源码构建镜像，不会拉取原作者预构建镜像。更新代码后务必带 `--build`，否则会沿用旧镜像、看不到新功能。

## 关键环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `UPSTREAMHUB_HTTP_PORT` | `8080` | Web 面板对外端口 |
| `APP_SECRET` | 空 | 必填，敏感字段加密主密钥；修改后既有加密数据无法解密 |
| `AUTH_ENABLED` | `true` | 是否开启登录鉴权 |
| `ADMIN_USERNAME` | `admin` | 初始管理员账号 |
| `ADMIN_PASSWORD` | 空 | 留空时使用 `admin/admin` 并强制首登改密 |
| `UPSTREAMHUB_SCHEDULER_SYNC_CRON` | `37 */3 * * * *` | 余额和倍率同步频率，6 字段 cron，含秒 |
| `UPSTREAMHUB_SCHEDULER_CONCURRENCY` | `4` | 同一轮最多并发扫描的上游数量 |
| `UPSTREAMHUB_NOTIFICATIONS_BATCH_RATE_CHANGES` | `true` | 倍率变化是否合并推送 |
| `UPSTREAMHUB_NOTIFICATIONS_MIN_CHANGE_PCT` | `0` | 倍率变化推送最小百分比阈值 |
| `UPSTREAMHUB_NOTIFICATIONS_RATE_CHANGE_DIRECTION` | `all` | 倍率变化推送方向：`all` / `increase` / `decrease` |
| `UPSTREAMHUB_NOTIFICATIONS_RATE_CHANGE_QUIET_GROUPS` | 空 | 逗号分隔的静默分组 |
| `UPSTREAMHUB_UPDATE_ENABLED` | `true` | 是否允许网页一键更新 |
| `UPSTREAMHUB_DEPLOY_BACKUP_INTERVAL_DAYS` | `7` | 部署前自动备份间隔；`0` 表示每次部署都备份 |

## 通知渠道配置

通知渠道的密钥、Webhook、SMTP 密码等敏感配置会加密保存。新增或编辑通知渠道时，按渠道类型填写对应 JSON 字段即可。

### Telegram

```json
{
  "bot_token": "1234567890:AAEh...",
  "chat_id": "-1001234567890"
}
```

### Webhook

```json
{
  "url": "https://example.com/hook",
  "method": "POST",
  "headers": {
    "Authorization": "Bearer xxx"
  }
}
```

### Email

```json
{
  "host": "smtp.example.com",
  "port": 465,
  "use_tls": true,
  "username": "alert@example.com",
  "password": "smtp-password-or-app-password",
  "from": "alert@example.com",
  "to": ["ops@example.com"]
}
```

### 企业微信

```json
{
  "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxx"
}
```

### 钉钉

```json
{
  "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
  "secret": "SEC..."
}
```

### 飞书

```json
{
  "webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
  "secret": "..."
}
```

### Server酱

```json
{
  "sendkey": "SCT...或 sctp..."
}
```

Turbo 版（`SCT` 开头）和 Server酱³（`sctp` 开头）会按 SendKey 前缀自动识别，无需额外选择版本。

## 订阅规则

通知渠道可以限制只接收指定上游或指定倍率分组的事件。留空或 `[]` 表示接收全部事件。

```json
[
  { "channel_id": 1, "mode": "all" },
  { "channel_id": 2, "mode": "groups", "groups": ["cc-max", "codex"] }
]
```

- `channel_id`：上游渠道 ID。
- `mode=all`：接收该上游全部事件。
- `mode=groups`：倍率变化只接收 `groups` 中指定的模型或分组。

## 运维中心

运维中心集中提供：

- 版本检查和网页一键更新。
- 更新阶段、进度条和实时日志尾部展示。
- 立即备份和备份下载。
- 手动同步余额、倍率，或同一轮同步余额和倍率。
- 失败通知重发。
- 诊断包下载和诊断摘要复制。
- 历史日志清理。

如果网页一键更新显示不可用，通常是当前容器缺少 Docker CLI、没有挂载 Docker socket，或 `.env` 中没有写入 `UPSTREAMHUB_UPDATE_HOST_DIR`。先在服务器运行一次 `./scripts/deploy.sh`，脚本会自动补齐更新需要的宿主机项目目录。

## 安全注意事项

- `APP_SECRET` 必须长期保存，丢失或更换后既有加密凭据无法解密。
- 生产环境建议设置强 `ADMIN_PASSWORD`，或首登后立即修改默认密码。
- 不建议在公网无反代保护的情况下关闭 `AUTH_ENABLED`。
- 通知配置、渠道凭据、打码平台密钥都会加密入库，但数据库和 `.env` 仍需妥善备份。
- 截图分享前注意渠道名称、余额、分组名称等业务信息；原站点地址默认隐藏，悬停时才会显示。

## 本地开发

前端：

```bash
cd frontend
npm install
npm run lint
npm run build
```

后端需要 Go 1.23 和 PostgreSQL。容器部署路径会在 Dockerfile 中完成前端构建、Go 构建和静态资源嵌入。

## 二次开发说明

本仓库是 [worryzyy/upstream-hub](https://github.com/worryzyy/upstream-hub) 的二次开发版本。核心监控能力来自原项目，本仓库主要新增和增强：

- Server酱通知渠道。
- 一键部署和一键更新。
- 默认登录鉴权、首登强制改密。
- 运维中心、备份、诊断、失败通知重发。
- 倍率变化通知策略和订阅规则。
- 余额趋势筛选和更清晰的监控面板。
- 更新提示、实时更新进度和部署前自动备份间隔。
- 截图友好的渠道原站点隐藏显示。

如果这个项目对你有帮助，也欢迎去给[原项目](https://github.com/worryzyy/upstream-hub)点个 Star。

## 致谢

- 原项目：[worryzyy/upstream-hub](https://github.com/worryzyy/upstream-hub) by [@worryzyy](https://github.com/worryzyy)

## License

沿用原项目协议：[原项目](https://github.com/worryzyy/upstream-hub) README 声明为 MIT。本仓库的二次开发改动同样以 MIT 协议发布。
