# LeetCodeClaw2C

LeetCodeClaw2C 是一个使用 Go 编写的 LeetCode 中文站题目抓取 CLI。它从 `leetcode.cn` 的公开接口按题目 `slug` 抓取题目正文、C/C++ 初始化代码、官方或公开社区题解中的 C/C++ 代码，并输出 Markdown 与 JSON 文件。

工具只读取公开可访问内容，不配置 Cookie，不绕过登录、会员或权限限制。

## 功能特性

- 按题目 `slug` 批量抓取 `leetcode.cn` 题目信息。
- 输出题目标题、难度、标签、题面、示例、约束等内容。
- 提取 C 和 C++ 初始化代码。
- 优先抓取官方题解，官方题解不满足时回退到公开社区题解。
- 题解成功标准为存在 C 或 C++ 任一可用代码块。
- 题目和题解中的图片会转换为 Markdown 图片链接，不下载到本地。
- 社区题解只读取文字正文，不下载视频，纯视频题解会跳过。
- 每题输出 `problem.md` 和 `problem.json`，便于阅读和后续程序处理。
- 单题失败不影响批量任务中的其他题目。

## 编译

在项目根目录执行：

```powershell
go build -o .\leetcode-claw.exe .\cmd\leetcodeclaw
```

编译成功后，当前目录会生成：

```text
leetcode-claw.exe
```

也可以直接使用 Go 运行：

```powershell
go run ./cmd/leetcodeclaw --slugs two-sum
```

## 快速开始

抓取单个题目：

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays
```

抓取多个题目：

```powershell
.\leetcode-claw.exe --slugs two-sum,median-of-two-sorted-arrays,longest-substring-without-repeating-characters
```

指定输出目录：

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays --out output
```

完整示例：

```powershell
.\leetcode-claw.exe `
  --slugs median-of-two-sorted-arrays,longest-substring-without-repeating-characters `
  --out output `
  --format md,json `
  --timeout 30s `
  --retries 3 `
  --delay 1s
```

PowerShell 中反引号 `` ` `` 表示换行续写。也可以写成一行：

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays,longest-substring-without-repeating-characters --out output --format md,json --timeout 30s --retries 3 --delay 1s
```

## 参数说明

| 参数 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--slugs` | 是 | 无 | 英文逗号分隔的题目 slug，例如 `two-sum,median-of-two-sorted-arrays`。 |
| `--out` | 否 | `output` | 输出目录。 |
| `--format` | 否 | `md,json` | 输出格式，支持 `md`、`json`、`md,json`。 |
| `--timeout` | 否 | `20s` | 单次 HTTP 请求超时时间。 |
| `--retries` | 否 | `2` | 瞬时网络失败重试次数。 |
| `--delay` | 否 | `500ms` | 多题抓取之间的等待间隔。 |

## 从 LeetCode 链接提取 slug

`--slugs` 参数需要的是题目 `slug`，不是完整 URL。

例如链接：

```text
https://leetcode.cn/problems/longest-substring-without-repeating-characters/solutions/
```

实际传入：

```powershell
.\leetcode-claw.exe --slugs longest-substring-without-repeating-characters
```

例如链接：

```text
https://leetcode.cn/problems/median-of-two-sorted-arrays/
```

实际传入：

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays
```

## 输出文件

每个成功抓取的题目会生成一个目录：

```text
output\<题目slug>\
```

目录内包含：

```text
problem.md
problem.json
```

`problem.md` 面向阅读，包含：

- 题目标题
- 题目 slug
- 难度
- 标签
- 题目正文
- C/C++ 初始化代码
- 题解正文
- C/C++ 题解代码
- 抓取警告

`problem.json` 面向程序处理，包含结构化字段：

- `questionId`
- `questionFrontendId`
- `title`
- `titleSlug`
- `translatedTitle`
- `difficulty`
- `tags`
- `contentMarkdown`
- `codeSnippets`
- `solution`
- `errors`

## 题解抓取规则

程序按以下顺序尝试抓取题解：

1. 当前题目的官方公开题解。
2. 题面中提到的主站等价题官方题解。
3. 当前题目的公开社区题解。
4. 等价题的公开社区题解。

成功标准：

- 有题解正文。
- 题解正文中存在 C 或 C++ 任一可用代码块。

也就是说：

- 只有 C 题解代码，可以成功。
- 只有 C++ 题解代码，可以成功。
- C 和 C++ 都有，可以成功。
- C 和 C++ 都没有，会失败。

如果题目最终失败，程序会清理该题目的旧输出目录，避免残留文件被误认为成功结果。

## 社区题解规则

当官方题解没有 C/C++ 代码时，程序会尝试公开社区题解：

- 优先选择带 C/C++ 标签的题解。
- 优先选择点赞、热度更高的题解。
- 只读取公开文字正文。
- 不下载视频。
- 纯视频且没有文字正文的题解会被跳过。

如果社区题解正文中出现图片，程序会保留为 Markdown 图片链接。

## 图片和视频处理

题目或题解中的图片会转换成 Markdown 图片格式：

```markdown
![alt](https://leetcode.cn/...)
```

或：

```markdown
![](https://assets.leetcode.cn/...)
```

程序不会把图片下载到本地，也不会下载视频。如果社区题解包含视频但同时有文字正文，程序只读取文字正文；如果是纯视频内容，程序会跳过该题解并继续尝试其他公开题解。

## 常见问题

### 为什么传完整 URL 不行？

当前 `--slugs` 参数只接受题目 slug。

错误示例：

```powershell
.\leetcode-claw.exe --slugs https://leetcode.cn/problems/two-sum/
```

正确示例：

```powershell
.\leetcode-claw.exe --slugs two-sum
```

### 为什么题目失败了？

常见原因：

- 题目 slug 写错。
- LeetCode 公开接口临时失败。
- 题解需要登录或会员权限。
- 官方和社区题解都没有 C/C++ 代码块。
- 网络超时或被限流。

失败时程序会在终端输出原因，例如：

```text
failed: 题解不完整，缺失 C/C++ 题解代码
```

### 如何降低限流风险？

可以增加多题之间的抓取间隔：

```powershell
.\leetcode-claw.exe --slugs two-sum,median-of-two-sorted-arrays --delay 2s
```

也可以增加超时时间和重试次数：

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays --timeout 30s --retries 3
```

### 如何只生成 Markdown？

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays --format md
```

### 如何只生成 JSON？

```powershell
.\leetcode-claw.exe --slugs median-of-two-sorted-arrays --format json
```

## 开发验证

运行测试：

```powershell
go test ./...
```

运行构建：

```powershell
go build ./...
```
