# LeetCodeClaw

`LeetCodeClaw` 是一个用 Go 编写的 LeetCode 中文站题目抓取与推荐 API 服务。服务访问 `leetcode.cn` 公开接口，支持按题目 slug 抓取题面与题解、根据关键词推荐候选题，并可将题目写入主项目 MySQL 题库表。

本项目现在只提供 HTTP API 服务，不再包含本地抓题 CLI 或压测 CLI。

## 功能

- 按题目 `slug` 抓取题面、难度、标签、C/C++ 初始化代码。
- 抓取官方题解，官方题解不完整时回退到公开社区题解。
- 根据关键词搜索候选题，并按标题、标签、难度进行轻量排序。
- HTTP API 默认监听 `:10170`。
- 可将题目写入主项目 MySQL：
  - `leetcode_problem_bank`
  - `leetcode_problem_tag`

## 配置

服务启动时会读取当前工作目录下的 `.env` 文件。系统环境变量优先级高于 `.env`，因此生产环境或 CI 注入的变量不会被 `.env` 覆盖。缺少 `.env` 时，服务会使用默认值启动。

可以复制 `.env.example` 为 `.env` 后按需修改：

```dotenv
LEETCODE_CLAW_ADDR=:10170
LEETCODE_CLAW_READ_TIMEOUT=30s
LEETCODE_CLAW_WRITE_TIMEOUT=120s
LEETCODE_CLAW_UPSTREAM_TIMEOUT=20s
LEETCODE_CLAW_RETRIES=2
LEETCODE_CLAW_DB_REQUIRED=false

DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=ptadatabase
DB_USERNAME=root
DB_PASSWORD=
```

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LEETCODE_CLAW_ADDR` | `:10170` | API 监听地址 |
| `LEETCODE_CLAW_READ_TIMEOUT` | `30s` | HTTP 读超时 |
| `LEETCODE_CLAW_WRITE_TIMEOUT` | `120s` | HTTP 写超时 |
| `LEETCODE_CLAW_UPSTREAM_TIMEOUT` | `20s` | LeetCode 上游请求超时 |
| `LEETCODE_CLAW_RETRIES` | `2` | LeetCode 上游请求重试次数 |
| `LEETCODE_CLAW_DB_REQUIRED` | `false` | 是否要求 MySQL 可用才允许启动 |
| `DB_HOST` | `127.0.0.1` | MySQL 主机 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_NAME` | `ptadatabase` | 数据库名 |
| `DB_USERNAME` | `root` | 数据库用户名 |
| `DB_PASSWORD` / `DB_PASS` | 空 | 数据库密码 |

## 启动

```powershell
go run ./cmd/leetcode-api
```

默认访问地址：

```text
http://127.0.0.1:10170
```

## API 接口

### 健康检查

```http
GET /health
```

返回服务状态、数据库连接状态和表结构检查结果。

### 按 slug 抓取

```http
POST /api/leetcode/crawl
Content-Type: application/json

{
  "slugs": ["two-sum"],
  "persist": true
}
```

`persist=true` 时写入 MySQL；`persist=false` 时只返回抓取结果。

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

服务会先调用 LeetCode 题库搜索接口获取候选题，再逐个抓取题面和题解。

### 查询本地题库

```http
GET /api/leetcode/problem/{slug}
```

默认只查询本地数据库。若希望查不到时实时抓取：

```http
GET /api/leetcode/problem/{slug}?crawl=true
```

## 数据库要求

API 写库时要求主项目数据库存在以下表和字段：

```text
leetcode_problem_bank:
  source_key, problem_code, numeric_id, title_main, title_alt,
  problem_text, solution_text, source_url, difficulty,
  estimated_minutes, quality_score

leetcode_problem_tag:
  problem_id, tag_type, tag_value, confidence
```

如果数据库不可达且 `LEETCODE_CLAW_DB_REQUIRED=false`，服务会降级启动，`/health` 中会显示数据库不可用；此时 `persist=true` 的请求会返回写库失败信息。

## 构建与测试

```powershell
go test ./...
go build ./cmd/leetcode-api
```

## 风险说明

- LeetCode GraphQL 字段不是稳定公共契约，字段变更时需要同步调整抓取逻辑。
- 不建议高并发调用关键词推荐或抓取接口，避免触发 `leetcode.cn` 限流。
- `persist=true` 会写入数据库，测试时建议先使用 `persist=false`。
