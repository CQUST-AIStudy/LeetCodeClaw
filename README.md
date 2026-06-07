# LeetCodeClaw

`LeetCodeClaw` 是一个使用 Go 编写的 LeetCode 中文站题目抓取与推荐 API 服务。服务访问 `leetcode.cn` 的公开接口，支持按题目 `slug` 抓取题面、题解、初始化代码，也支持关键词推荐候选题，并可将结果写入 MySQL 题库表。

## 功能概览

- 按 `slug` 抓取题面、难度、标签、初始化代码和题解。
- 优先抓取官方题解，官方题解不完整时回退到公开社区题解。
- 根据关键词搜索候选题，并按标题、标签、难度进行轻量排序。
- 支持后台全量抓取 LeetCode 中文站公开题库。
- 支持写入 MySQL 表 `leetcode_problem_bank`、`leetcode_problem_tag`。
- 支持 Docker Compose 一键启动 API 与 MySQL。
- 支持可选 API Key 鉴权和 CORS 来源白名单。

## Docker 快速启动

推荐使用 Docker Compose 部署 API 和 MySQL。容器内 API 连接 MySQL 时必须使用服务名 `mysql` 和容器端口 `3306`，不能使用 `127.0.0.1`。

1. 复制 Docker 环境模板：

```powershell
Copy-Item .env.docker.example .env.docker
```

2. 修改 `.env.docker`：

- 将 `MYSQL_ROOT_PASSWORD` 改为强密码。
- 将 `DB_PASSWORD` 改为强密码。
- 将 `LEETCODE_CLAW_API_KEY` 改为生产可用的随机密钥。
- 按需收紧 `LEETCODE_CLAW_CORS_ORIGINS`。

3. 启动服务：

```powershell
docker compose --env-file .env.docker up -d --build
```

默认宿主机访问地址：

```text
API:   http://127.0.0.1:10170
MySQL: 127.0.0.1:3307
```

查看状态：

```powershell
docker compose --env-file .env.docker ps
docker compose --env-file .env.docker logs -f leetcode-api
```

停止服务：

```powershell
docker compose --env-file .env.docker down
```

如需删除 MySQL 数据卷并重新初始化表结构：

```powershell
docker compose --env-file .env.docker down -v
```

## 本地开发启动

本地运行时可复制 `.env.example`：

```powershell
Copy-Item .env.example .env
go run ./cmd/leetcode-api
```

默认监听：

```text
http://127.0.0.1:10170
```

如果 MySQL 在 Docker 中运行且映射到宿主机 `3307`，本地 `.env` 可使用：

```dotenv
DB_HOST=127.0.0.1
DB_PORT=3307
```

如果 API 也运行在 Docker 容器中，应使用：

```dotenv
DB_HOST=mysql
DB_PORT=3306
```

## 配置说明

服务启动时会读取当前工作目录下的 `.env`。系统环境变量优先级高于 `.env`，因此生产环境或 CI 注入的变量不会被本地文件覆盖。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LEETCODE_CLAW_ADDR` | `:10170` | API 监听地址 |
| `LEETCODE_CLAW_READ_TIMEOUT` | `30s` | HTTP 读超时 |
| `LEETCODE_CLAW_WRITE_TIMEOUT` | `120s` | HTTP 写超时 |
| `LEETCODE_CLAW_UPSTREAM_TIMEOUT` | `20s` | LeetCode 上游请求超时 |
| `LEETCODE_CLAW_RETRIES` | `2` | LeetCode 上游请求重试次数 |
| `LEETCODE_CLAW_DB_REQUIRED` | `false` | 是否要求 MySQL 可用才允许启动；生产建议设为 `true` |
| `LEETCODE_CLAW_CRAWL_ALL_WORKERS` | `1` | 全量抓取 worker 数 |
| `LEETCODE_CLAW_CRAWL_ALL_PAGE_SIZE` | `100` | 全量枚举题库每页数量，最大 `200` |
| `LEETCODE_CLAW_CRAWL_ALL_DELAY` | `2s` | 每个 worker 抓取单题后的等待时间 |
| `LEETCODE_CLAW_API_KEY` | 空 | 非空时保护 `/api/` 业务接口 |
| `LEETCODE_CLAW_CORS_ORIGINS` | `*` | 允许的 CORS 来源，多个来源用英文逗号分隔 |
| `DB_HOST` | `127.0.0.1` | MySQL 主机 |
| `DB_PORT` | `3307` | MySQL 端口 |
| `DB_NAME` | `ptadatabase` | 数据库名 |
| `DB_USERNAME` | `root` | 数据库用户名 |
| `DB_PASSWORD` / `DB_PASS` | 空 | 数据库密码 |

## 健康检查

```http
GET /health
```

返回服务、数据库和表结构诊断信息。该接口始终用于观测，数据库不可用时也可能返回 HTTP `200`。

```http
GET /ready
```

用于 Docker healthcheck 和就绪探测。只有 MySQL 可连接且表结构检查通过时返回 HTTP `200`，否则返回 HTTP `503`。

## API 鉴权

当 `LEETCODE_CLAW_API_KEY` 为空时，保持兼容，业务接口无需鉴权。

当 `LEETCODE_CLAW_API_KEY` 非空时，所有 `/api/` 业务接口需要提供以下任意一种凭据：

```http
Authorization: Bearer your-api-key
```

或：

```http
X-API-Key: your-api-key
```

`/health`、`/ready` 和 `OPTIONS` 预检请求不需要 API Key。

## API 接口

### 按 slug 抓取

```http
POST /api/leetcode/crawl
Content-Type: application/json

{
  "slugs": ["two-sum"],
  "persist": true
}
```

- `slugs` 必填，支持一次传入多个题目 slug。
- `persist` 默认 `true`。
- `persist=true` 时会写入 MySQL。
- `persist=false` 时只返回抓取结果，不写入数据库。

### 全量抓取公开题库

```http
POST /api/leetcode/crawl/all
Content-Type: application/json

{
  "persist": true,
  "forceRefresh": true
}
```

- 同一时间只允许一个全量任务运行。
- 只枚举 `leetcode.cn` 公开题库，跳过付费或不可见题目。
- 任务状态写入 `leetcode_crawl_job` 和 `leetcode_crawl_job_item`。

查询任务进度：

```http
GET /api/leetcode/crawl/jobs/{jobId}
```

任务状态可能为 `queued`、`running`、`succeeded`、`partial_failed`、`failed`、`canceled`。

### 关键词推荐候选题

```http
POST /api/leetcode/recommend/keyword
Content-Type: application/json

{
  "keyword": "动态规划",
  "limit": 10,
  "difficulty": "Medium",
  "persist": false
}
```

`limit` 最大为 `50`。建议保持较小值，避免触发上游限流。

### 查询本地题库

```http
GET /api/leetcode/problem/{slug}
```

默认只查本地数据库。如需本地查不到时实时抓取：

```http
GET /api/leetcode/problem/{slug}?crawl=true
```

## 数据库说明

Docker Compose 首次初始化 MySQL 数据卷时，会执行 `deploy/mysql/init/001_leetcode_schema.sql` 创建题库表。若数据卷已经存在，MySQL 不会重复执行初始化脚本。

写库要求以下核心字段存在：

```text
leetcode_problem_bank:
  id, source_key, problem_code, numeric_id, title_main, title_alt,
  problem_text, solution_text, code_snippets_json, source_url,
  difficulty, estimated_minutes, quality_score

leetcode_problem_tag:
  id, problem_id, tag_name, tag_category, relevance_score, is_primary
```

全量抓取任务表会由服务启动时自动创建：

```text
leetcode_crawl_job
leetcode_crawl_job_item
```

生产环境如不希望应用启动时自动调整表结构，请改为通过数据库迁移流程管理 schema。

## 构建与测试

```powershell
go test ./...
go build ./cmd/leetcode-api
docker compose --env-file .env.docker config
```

如需先用模板校验 Compose：

```powershell
docker compose --env-file .env.docker.example config
```

## 安全与风险

- 生产部署建议设置 `LEETCODE_CLAW_DB_REQUIRED=true` 和 `LEETCODE_CLAW_API_KEY`。
- 不建议将 API 直接公网裸露；请放在内网、VPN、反向代理鉴权或 API 网关后。
- `LEETCODE_CLAW_CORS_ORIGINS=*` 适合本地调试，生产建议改为明确域名。
- LeetCode GraphQL 字段不是稳定公开契约，字段变更时需要同步调整抓取逻辑。
- 不建议高并发调用抓取或推荐接口；默认全量抓取使用 `1` 个 worker 和 `2s` 延迟以降低限流风险。
- `persist=true` 会写入数据库，调试时建议先使用 `persist=false`。

## 常见问题

### API 容器连不上 MySQL

确认 Docker Compose 中 API 使用的是：

```dotenv
DB_HOST=mysql
DB_PORT=3306
```

`127.0.0.1` 在 API 容器内表示 API 容器自身，不是 MySQL 容器。

### 修改初始化 SQL 后没有生效

MySQL 官方镜像只会在空数据目录首次启动时执行 `/docker-entrypoint-initdb.d`。本地调试可删除数据卷后重新启动：

```powershell
docker compose --env-file .env.docker down -v
docker compose --env-file .env.docker up -d --build
```

### `/ready` 返回 503

检查 MySQL 是否健康、`.env.docker` 密码是否一致，以及 `leetcode_problem_bank`、`leetcode_problem_tag` 表结构是否完整。
