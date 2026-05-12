# Goploy CLI & MCP Server (`goploy-cli`)

## Context

Goploy 今天只能通过 Web UI 或裸调 REST API 触发部署。用户希望让 AI agent（Claude Code / Cursor 等）用自然语言完成日常发布，例如输入 `发布 far-boo`，agent 就自动解析项目名、触发 `/deploy/publish`、轮询 `/deploy/getPublishProgress` 直到成功或失败——等价于原本在 Web UI 点"发布"按钮 + 盯进度条。

同时，**Goploy 服务器和使用 agent 的用户往往不是同一台机器**。其他用户本地没有 goploy 源代码，他们需要的只是一个"能装上就能用"的客户端：通过 `GOPLOY_URL` + `GOPLOY_API_KEY` + `GOPLOY_NAMESPACE_ID` 指向远端实例。因此方案必须是一个**独立发布的工件**（npm 包），而不是在 server 代码里加 handler。

目标产物：
1. 一个 **MCP server**（agent 原生协议）—— 让任何 MCP 兼容的 agent 获得结构化工具
2. 一个 **CLI**（同一个 bin 的子命令）—— 用于 shell 脚本 / CI / 人工调试
3. 一个 **Claude Code skill**（markdown）—— 教 agent 如何编排这些工具（名字消歧、轮询、失败取日志、二次确认）

## Decisions (已确认)

| 维度 | 选择 |
|------|------|
| 仓库 | **独立仓库 `github.com/goploy-devops/goploy-cli`**（新建 org + repo） |
| License | **MIT**（client 给生态更大自由度，不传染 goploy 主仓库的 GPLv3） |
| npm 包名 | `goploy-cli`（无 scope；发布前 `npm view goploy-cli` 确认 404） |
| bin 命令名 | `goploy`（短好记，类似 `kubectl`、`gh`） |
| 运行时 | **Node.js 18+ / TypeScript**（MCP SDK 成熟、`npx` 零安装） |
| 工具范围（首版） | **MVP 发布闭环**：list/resolve projects、publish、wait_for_publish、get_publish_trace、rebuild、history、reset_state |
| 首版本号 | `v0.1.0`，独立于 goploy server 版本号 |

**不做**（留给下一版）：审批流 `review`、进程管理 `manageProcess`、文件 diff、WebSocket 实时推送（用 HTTP 轮询 `/deploy/getPublishProgress` 2–3s 间隔够用）、多 namespace / 多实例并存。

## Architecture

```
┌─────────────────────┐       stdio MCP      ┌───────────────────────┐        HTTPS         ┌──────────────┐
│  Claude Code /      │◀────────────────────▶│  goploy-cli (node)    │─────────────────────▶│ Goploy Server│
│  Cursor / ...       │                      │                       │   X-API-KEY +G-N-ID  │  (远端)      │
└─────────────────────┘                      │  bin: goploy          │                      └──────────────┘
                                             │  ├─ sub: mcp          │
┌─────────────────────┐                      │  ├─ sub: publish      │
│  Shell / CI         │───── exec bin ──────▶│  ├─ sub: status|wait  │
└─────────────────────┘                      │  ├─ sub: trace        │
                                             │  ├─ sub: rebuild      │
                                             │  └─ sub: ls|history   │
                                             └───────────────────────┘
```

**一个包，两个前端。** `goploy mcp` 启动 stdio MCP server；`goploy publish far-boo --branch main` 是直接 CLI 调用。**两者共享同一个 `src/client/`**，HTTP 客户端、错误类型、轮询逻辑只写一份。

### 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| MCP SDK | `@modelcontextprotocol/sdk` | 官方 TS SDK，stdio transport 内置 |
| CLI | `commander` | 成熟、轻量、tsx 调试友好 |
| HTTP | 原生 `fetch` (Node 18+) | 零依赖；避免 axios 体积 |
| Schema | `zod` | MCP 工具参数 schema 可直接复用为 TS 类型 |
| 打包 | `tsup` | 产出单文件 bin，`npx` 启动 <500ms |
| 测试 | `vitest` + `msw` | `msw` mock Goploy HTTP 响应 |
| 发布 | npm public 包 `goploy-cli` | `npx -y goploy-cli mcp` 零安装 |

Node 18+ 硬门槛（`fetch` + `AbortSignal.timeout`）。CI 跑 Node 18/20 矩阵 + `windows-latest` smoke。

### 新仓库目录结构

```
goploy-cli/                             # github.com/goploy-devops/goploy-cli
  package.json                          # name: "goploy-cli", bin: { "goploy": "dist/cli.js" }
  tsconfig.json
  tsup.config.ts
  LICENSE                               # MIT
  README.md                             # 安装、env vars、Claude Code 接入、例子
  CHANGELOG.md                          # 记录兼容的 goploy server 版本区间
  .github/workflows/
    ci.yml                              # lint + test, Node 18/20 on ubuntu + windows
    release.yml                         # tag push → npm publish --access public
  src/
    cli.ts                              # #!/usr/bin/env node, commander 分发
    mcp.ts                              # MCP server: 注册 zod schema 工具
    commands/
      publish.ts / status.ts / wait.ts / trace.ts / rebuild.ts /
      ls.ts / history.ts / reset.ts / config.ts
    client/
      index.ts                          # GoployClient: 注入 X-API-KEY + G-N-ID
      deploy.ts                         # publish / rebuild / getProgress / getTrace / getTraceDetail / getPreview / resetState
      project.ts                        # getList + resolveProject(query) 模糊匹配
      types.ts                          # 响应 DTO（起点 copy 自 goploy 主仓 web/src/api/deploy.ts）
      errors.ts                         # NetworkError / AuthError / NotFoundError / BusinessError
    agent/
      tools.ts                          # 注册 MCP 工具（thin wrapper over client）
      poll.ts                           # waitForPublish: 轮询 + 超时 + 取消
      skill.md                          # Claude Code skill 原文（打包 embed 进 bin）
    config.ts                           # env var 读取 + 校验
  test/
    client.deploy.test.ts
    client.project.test.ts
    agent.poll.test.ts
  docs/
    skill.md                            # 单文件可复制到 ~/.claude/skills/goploy.md
```

## MCP Tool Surface (MVP)

| Tool | 参数 (zod) | 返回 | 备注 |
|------|-----------|------|------|
| `list_projects` | `keyword?: string` | `[{id, name, branch, autoDeploy, lastPublishState}]` | 模糊匹配；空 keyword 返回全部 |
| `resolve_project` | `query: string` | 1 命中 → `{id, name}`；多命中 → 候选列表 | `publish` 内部也调用 |
| `publish` | `project: string\|int64, branch?, commit?, server_ids?: int64[]` | `{token, projectId, projectName}` | 名字解析 → `/deploy/publish`；**不轮询** |
| `get_publish_status` | `token: string` | `{state: "deploying"\|"success"\|"fail", stage, message}` | 1 次快照 |
| `wait_for_publish` | `token: string, timeout_sec?=600, poll_interval_sec?=3` | `{final_state, stage, message, trace_summary?}` | 内部轮询；fail 附 trace 摘要 |
| `get_publish_trace` | `token: string, include_detail?: boolean` | `[{id, type, state, detail?, ext}]` | 失败诊断 |
| `rebuild` | `token: string` | `{type: "symlink"\|"publish", new_token}` | 回滚 |
| `list_recent_deployments` | `project_id?, limit?=20, state?` | 分页历史 | 封装 `/deploy/getPreview` |
| `reset_project_state` | `project_id: int64` | ok/err | 解锁卡住项目 |

**"一键发布"组合模式（skill 里写死）：**
`resolve_project` → 跟用户确认 → `publish` → `wait_for_publish` → fail 则 `get_publish_trace(include_detail=true)` → 一句话回报。

## CLI Surface

| 命令 | 说明 |
|------|------|
| `goploy mcp` | 启动 MCP server（stdio） |
| `goploy ls [--keyword X]` | 列项目 |
| `goploy publish <project> [--branch B] [--commit C] [--servers 1,2,3] [--wait]` | `--wait` 阻塞到终态 |
| `goploy status <token>` | 一次性查状态 |
| `goploy wait <token> [--timeout 600]` | 轮询到终态 |
| `goploy trace <token> [--detail]` | 看日志 |
| `goploy rebuild <token>` | 回滚 |
| `goploy history <project> [--limit 20]` | 历史 |
| `goploy reset <project>` | 解锁 |
| `goploy config check` | 打印 URL/namespace，测试鉴权 |
| `goploy install-skill` | 把 `src/agent/skill.md` 写入 `~/.claude/skills/goploy.md` |

全局 flags：`--url` / `--api-key` / `--namespace`（env var 优先：`GOPLOY_URL` / `GOPLOY_API_KEY` / `GOPLOY_NAMESPACE_ID`）。

## Distribution & Install

**npm registry** — 包名 `goploy-cli`(无 scope，发布前 `npm view goploy-cli` 确认可用）。发布命令：`npm publish --access public`。

**最终用户三条安装路径：**

```bash
# 1) 全局 CLI 用
npm i -g goploy-cli
goploy config check

# 2) npx 免安装（推荐给 MCP）
npx -y goploy-cli config check

# 3) 贡献者从源码
git clone https://github.com/goploy-devops/goploy-cli
cd goploy-cli && npm i && npm run build && npm link
```

**接入 Claude Code：**

```bash
export GOPLOY_URL=https://goploy.example.com
export GOPLOY_API_KEY=xxx        # PUT /user/generateApiKey 在 goploy UI 生成
export GOPLOY_NAMESPACE_ID=1

claude mcp add goploy -- npx -y goploy-cli mcp
goploy install-skill             # 可选，自动装好 Claude Code skill
```

## Security & Correctness

- **API key 传递**：只走 env var 或 `~/.config/goploy-cli/config.json`（启动时检查 mode 0600，否则 warn）；**绝不**出现在命令行参数（避免 shell history）
- **TLS**：默认校验证书；允许 `GOPLOY_INSECURE_SKIP_VERIFY=1` 走自签名内网（通过 `undici` Dispatcher 传入）
- **幂等保护**：`publish` 返回 token 后，skill 要求 agent **在同一会话内**不对同一 project 重复触发 `publish`，除非用户明确要求
- **超时**：`wait_for_publish` 默认 10 分钟上限；超时不杀服务端部署，只返回 `"still_deploying"` 并附最后一次 status
- **错误映射**：HTTP 2xx + 业务 code != 0 → `BusinessError`；401/403 → `AuthError`（提示检查 API key / namespace / 权限）；404 项目找不到 → `NotFoundError`（引导 agent 跑 `list_projects` 消歧）；网络/超时 → `NetworkError`（提示检查 URL / VPN）
- **namespace 缺省**：`G-N-ID` 未设置调用会失败；`goploy config check` 一次性验证（调 `/project/getList` 打水漂）并给出可读错误
- **日志**：默认只打 ERROR 到 stderr；`GOPLOY_DEBUG=1` 打 HTTP 请求摘要（脱敏 API key）

## Cross-repo Coordination

新仓库 `goploy-devops/goploy-cli` 与主仓 `zhenorzz/goploy` 的接口是 HTTP API，不共享代码。需要做的少量联动：

**在 `zhenorzz/goploy` 主仓库（本仓库）一次性改动：**
- `README.md` / `README-zh-CN.md`：新增 "CLI / MCP" 一节，指向 `https://github.com/goploy-devops/goploy-cli`
- `AGENTS.md`：在 "Environment Variables" 表格下方加一行注释："`GOPLOY_URL` / `GOPLOY_API_KEY` / `GOPLOY_NAMESPACE_ID` 由独立客户端 `goploy-cli` 使用"

**在 `goploy-cli` 新仓库里做 schema 同步：**
- `src/client/types.ts` 起点：`curl https://raw.githubusercontent.com/zhenorzz/goploy/master/web/src/api/deploy.ts` 复制字段
- `README.md` 声明 "Tested against Goploy ≥ 1.17.5"
- CI 里跑一个 integration job：`docker compose up goploy` + 真实调用做 smoke

## Testing

- **Unit** (`test/*.test.ts`)：`vitest` + `msw`，覆盖 auth header 注入、错误映射、namespace 透传、`waitForPublish` 轮询超时 + 取消、`resolveProject` 消歧
- **MCP schema**：对每个工具 `zod.safeParse(invalidInput)` 应拒绝；`npx @modelcontextprotocol/inspector goploy mcp` 手工验证
- **Integration**（可选，需真实 goploy 或 docker-compose）：`npm run test:int`，读 `.env.test`
- **E2E smoke**（写在 README）：
  1. `goploy config check` → server 版本 + namespace 名
  2. `goploy ls` → 至少一个项目
  3. `goploy publish <test_project> --branch master --wait` → success
  4. `goploy trace <token>` → 看日志
  5. Claude Code 里说"发布 <test_project>" → agent 全程跑通

## Rollout

1. **Phase 0** — 创建 GitHub org `goploy-devops` 和空仓库 `goploy-cli`；抢 npm 包名 `goploy-cli`（发一个 `0.0.0-placeholder`）
2. **Phase 1** — `src/client/` + unit tests（先把 HTTP 层打磨干净）
3. **Phase 2** — CLI 子命令（最小闭环：`ls` / `publish --wait` / `trace` / `config check`）
4. **Phase 3** — MCP server + skill markdown + `goploy install-skill` 子命令
5. **Phase 4** — CI + `v0.1.0` 正式 npm 发布；写 README；主仓 README 加指向链接
6. **Phase 5** — 用 Claude Code 真跑一次 "发布 far-boo"，根据体验回头调 skill 措辞和错误消息

## Open Risks

- **版本兼容性**：goploy server 如果改 `/deploy/publish` 请求/响应字段，client 会悄悄坏。缓解：CI integration job 对最新 release 跑 smoke；CHANGELOG 明确声明 "compatible server versions"
- **npm 包名抢注**：发布前务必 `npm view goploy-cli` 确认 404；若被占用退到 `@goploy-devops/cli`（此时要在 npm 建同名 org）
- **Windows 用户**：`npx` 在 Windows 有时路径解析有问题 —— CI 矩阵加 `windows-latest` 跑 smoke
- **API key 从哪来**：首次使用前用户必须先登录 Goploy UI 生成 API key（PUT `/user/generateApiKey`）—— 在 README 里给截图或 curl 示例，降低上手门槛
