# Upstream Hub

面向 NewAPI / Sub2API 站点的上游渠道监控面板，用来集中查看余额、模型倍率、倍率变化记录和通知状态。

> 本项目基于 [worryzyy/upstream-hub](https://github.com/worryzyy/upstream-hub) 二次开发，感谢原作者 [@worryzyy](https://github.com/worryzyy) 的开源工作。本仓库在其基础上新增了 Server酱 通知渠道、一键部署脚本，以及默认开启登录 + 首次强制改密等功能，详见文末「二次开发说明」。

## 预览

![Upstream Hub 预览 1](docs/images/demo1.png)

![Upstream Hub 预览 2](docs/images/demo2.png)

![Upstream Hub 预览 3](docs/images/demo3.png)

![Upstream Hub 预览 4](docs/images/demo4.png)

主要能力：

- 多上游渠道管理
- 余额汇总和低余额提醒
- 模型倍率监控和变化记录
- Cloudflare Turnstile 打码支持
- Telegram、Webhook、邮件、企业微信、钉钉、飞书、Server酱 通知

## 启动方式

### 一键部署（推荐）

新服务器上**一条命令**搞定（自动 clone / 更新 / 构建 / 启动）：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/upstream-hub/main/scripts/bootstrap.sh | bash
```

它会自动：clone 仓库（已存在则更新）→ 检查 docker / compose → 首次生成 `.env`（随机 `APP_SECRET` / `POSTGRES_PASSWORD`，默认开启登录）→ `docker compose up -d --build` 构建并启动 → 等待健康检查。

> 前提：服务器已装好 `git`、`docker`、`docker compose`、`curl`。

启动后访问 `http://localhost:8080`，**默认账号 `admin` / `admin`，首次登录会强制要求修改密码。**

#### 已经 clone 过仓库？

进到目录里跑部署脚本即可，每次更新也是这一条：

```bash
cd upstream-hub
./scripts/deploy.sh
```

### 手动 Docker Compose

```bash
cp .env.example .env
```

编辑 `.env`，至少设置：

```env
APP_SECRET=请替换为 32 字节以上随机字符串
POSTGRES_PASSWORD=请替换为数据库密码
```

鉴权默认开启（`AUTH_ENABLED=true`）。默认账号 `admin` / `admin`，首次登录强制改密；凭据持久化在数据库（bcrypt 哈希）。如想直接指定一个强密码、跳过首登改密：

```env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请替换为强密码
```

纯内网 / 反代后面想免登录，可设 `AUTH_ENABLED=false`。

启动：

```bash
docker compose up -d --build
```

启动后访问：

```text
http://localhost:8080
```

> 说明：本仓库是二开版，`docker-compose.yml` 会从当前源码构建镜像（包含本仓库的二开改动），不会去拉原作者的预构建镜像。**更新代码后务必带 `--build` 重新构建**（`docker compose up -d --build`，或直接用 `./scripts/deploy.sh`），否则会沿用旧镜像、看不到新功能。

## 通知渠道配置

通知渠道的密钥、Webhook、SMTP 密码等敏感配置会加密保存。新增或编辑通知渠道时，按渠道类型填写对应字段即可。

### Telegram

```json
{
	"bot_token": "1234567890:AAEh...",
	"chat_id": "-1001234567890"
}
```

- `bot_token`：从 Telegram 的 `@BotFather` 创建机器人后获取。
- `chat_id`：接收消息的私聊、群组或频道 ID。

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

- `url` 必填。
- `method` 默认 `POST`，也可以填 `PUT` 或 `GET`。
- `headers` 可选，用于自定义请求头。

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

- `host`、`port`、`from`、`to` 必填。
- `username`、`password` 取决于 SMTP 服务商是否要求鉴权。
- 常见端口：`465` 通常配合 `use_tls=true`，`587` 通常配合 STARTTLS。

### 企业微信

```json
{
	"webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxx"
}
```

填写群机器人的完整 Webhook URL。

### 钉钉

```json
{
	"webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
	"secret": "SEC..."
}
```

- `webhook_url` 必填。
- `secret` 可选，启用机器人“加签”时填写。

### 飞书

```json
{
	"webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
	"secret": "..."
}
```

- `webhook_url` 必填。
- `secret` 可选，启用“签名校验”时填写。

### Server酱

```json
{
	"sendkey": "SCT...或 sctp..."
}
```

- `sendkey` 必填，从 [Server酱·Turbo版](https://sct.ftqq.com/) 或 [Server酱³](https://sc3.ft07.com/) 的 SendKey 页面获取。
- Turbo 版（`SCT` 开头）和 Server酱³（`sctp` 开头）会按 SendKey 前缀自动识别，无需选择版本。
- 通知会作为微信 / APP 推送送达，`title` 取消息标题、`desp` 取正文（支持 Markdown）。

### 订阅规则

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

## 二次开发说明

本仓库是 [worryzyy/upstream-hub](https://github.com/worryzyy/upstream-hub) 的二次开发版本。核心监控能力（多上游管理、余额/倍率监控、打码、通知调度等）均来自原项目，在此基础上新增：

- **Server酱通知渠道** — 支持 Turbo 版与 Server酱³，按 SendKey 前缀自动识别。
- **一键部署脚本** [`scripts/deploy.sh`](scripts/deploy.sh) — 自动生成 `.env`、拉取代码、构建并启动、健康检查。
- **默认开启后台登录** — 默认账号 `admin` / `admin`，凭据持久化在数据库（bcrypt），首次登录强制修改密码。

如果这个项目对你有帮助，也欢迎去给[原项目](https://github.com/worryzyy/upstream-hub)点个 Star ⭐。

## 致谢

- 原项目：[worryzyy/upstream-hub](https://github.com/worryzyy/upstream-hub) by [@worryzyy](https://github.com/worryzyy)

## License

沿用原项目协议：[原项目](https://github.com/worryzyy/upstream-hub) README 声明为 MIT。本仓库的二次开发改动同样以 MIT 协议发布。
