# Mneme - Go 版

为 Claude Code 打造的高性能语义代码搜索工具，基于 Go 实现，支持可插拔的向量存储后端。

## 特性

- **语义搜索** - 使用自然语言查询定位相关代码
- **多后端支持** - chromem-go（纯 Go）、DuckDB（嵌入式）或 Qdrant（高扩展）
- **多嵌入模型** - 支持 SiliconFlow 和 Qwen
- **单一二进制** - chromem 和 DuckDB 后端无需额外基础设施
- **MCP 集成** - 通过 stdio JSON-RPC 与 Claude Code 直接协作
- **Hook 系统** - 轻量级钩子追踪编辑、会话和上下文使用情况
- **Anatomy Map** - 增量文件扫描，为 Claude 构建项目摘要
- **会话记忆** - 自动总结历史会话，节省上下文 token
- **Cerebrum** - 项目级编码规则，以 Claude 警告的形式呈现
- **Buglog** - 项目级 Bug 历史，与新编辑自动匹配

## 快速开始

### 前提条件

- Go 1.21+
- SiliconFlow 或 Qwen API 密钥

### 构建

```bash
# 默认构建（chromem 后端，无需 CGO）
go build -o ./bin/mneme ./cmd

# 带 DuckDB 后端构建（需要 CGO）
CGO_ENABLED=1 go build -tags duckdb -o ./bin/mneme ./cmd
```

### 初始化项目

在任意 git 仓库中运行：

```bash
cd /path/to/your/project
mneme init --yes
```

此命令会在 Claude Code 的 `settings.json` 中安装五个钩子，将所有源文件扫描到 anatomy map（`.mneme/anatomy.md`），并将项目上下文规则写入 `~/.claude/CLAUDE.md`。

### 注册到 Claude Code（MCP 服务器）

```bash
claude mcp add mneme \
  -e EMBEDDING_API_KEY=$EMBEDDING_API_KEY \
  -e EMBEDDING_PROVIDER=$EMBEDDING_PROVIDER \
  -- /path/to/bin/mneme
```

注册完成后，Claude Code 即可对代码库进行语义索引和搜索，无需额外配置。

## 功能教程

### Anatomy Scan

Anatomy map 是一个 Markdown 格式的源文件索引：记录每个文件的语言、预估 token 数和一行内容摘要。Claude 在打开文件前会先读取它，从而判断是否需要打开该文件。

**首次扫描（`init` 时自动执行）：**

```bash
mneme scan
# ✓ Scanned 142 files → .mneme/anatomy.md
```

**增量扫描（快速，默认模式）：**

后续运行时，仅重新提取上次扫描后修改过的文件，未变更的文件从缓存的 anatomy 中读取。干净仓库通常在 2 秒内完成。

```bash
mneme scan
# ✓ Scanned 3 files → .mneme/anatomy.md
```

**强制全量扫描：**

```bash
mneme scan --force
# ✓ Scanned 142 files → .mneme/anatomy.md
```

大规模重构或文件重命名后，运行 `scan` 以保持 map 的准确性。

---

### 会话记忆

每次 Claude Code 会话结束时，钩子会将结构化的摘要行写入 `~/.claude/mneme-memory.md`。此文件会被注入到后续会话中，让 Claude 无需重新阅读代码即可了解你的工作历史。

一行记录如下：

```
## 2026-04-20T09:15:00Z (12 turns)
Files: cmd/cmd_scan.go (feature×2), pkg/scanner/extractor.go (refactor×1)
Patterns: feature×2, refactor×1
Summary: Added 2 feature(s). Refactored 1 area(s).
```

**查看最近会话：**

```bash
mneme stats
```

输出会显示 `=== Recent Sessions ===` 下的最近五条会话记录。

**手动合并：**

当记忆文件超过 50 行（大约两个月的日常使用），session-start 钩子会自动合并。你也可以手动触发：

```bash
mneme memory consolidate
# consolidated 37 session row(s)
```

旧会话（超过 7 天）会被折叠成一条引用块，在保留近期信息的同时避免膨胀上下文：

```
> Consolidated session (284 actions from 37 sessions before 2026-04-13)
```

---

### 统计信息

查看当前项目的钩子活动、编辑模式和记忆行数：

```bash
mneme stats
```

示例输出：

```
project: abc123-myrepo

=== Hook Totals ===
  pre-read:                14
  pre-write:               8
  session-start:           3
  stop:                    3
  hook_errors:             0

=== Edit Patterns ===
  feature:             5
  refactor:            3
  bugfix:              1

=== Recent Sessions (last 5) ===
  2026-04-20T09:15:00Z         12 turns  feature×2, refactor×1
  2026-04-19T14:03:22Z          7 turns  bugfix×1
```

**上下文浪费诊断** — 检测重复读取、大文件未变更等浪费上下文的模式：

```bash
mneme stats --waste
```

**JSON 输出**，用于脚本或仪表盘：

```bash
mneme stats --json
```

---

### Cerebrum（编码规则）

Cerebrum 允许你为项目附加基于正则的警告规则。当 Claude 即将写入匹配规则的文件时，pre-write 钩子会在响应中注入警告消息——在编辑前提醒 Claude 注意项目规范。

**添加规则：**

```bash
mneme cerebrum add \
  --pattern "pkg/state/" \
  --message "所有状态变更必须通过 AtomicWrite；禁止直接写文件。" \
  --comment "State 包写入守卫"
```

**列出规则：**

```bash
mneme cerebrum list
# 1  pkg/state/  →  所有状态变更必须通过 AtomicWrite；禁止直接写文件。
#    (State 包写入守卫)
```

**删除规则：**

```bash
mneme cerebrum remove 1 --yes
```

规则存储在项目内的 `.mneme/cerebrum.json` 中。

---

### Buglog

Buglog 是项目级的 Bug 历史记录。当 Claude 即将写入代码时，pre-write 钩子会检查变更行是否匹配已知的 Bug 模式，如果匹配则在钩子输出中附加提醒。

**手动记录 Bug：**

```bash
mneme buglog add \
  --description "忘记在写入 memory.md 前获取锁" \
  --code "state.AtomicWrite(memPath, data)" \
  --file "pkg/state/memory.go"
```

**列出已记录的 Bug：**

```bash
mneme buglog list
# 1  2026-04-18  pkg/state/memory.go
#    忘记在写入 memory.md 前获取锁
```

**post-write 钩子自动记录：**

当 Claude 写入的提交被分类为 bugfix 时，post-write 钩子会自动创建 buglog 条目。无需手动操作即可积累 Bug 历史。

**清除记录：**

```bash
mneme buglog clear --yes
```

---

### MCP 工具（高级）

作为 MCP 服务器运行时，mneme 除了 index/search 外还暴露三个额外工具：

| 工具 | 说明 |
|---|---|
| `describe_codebase` | 以结构化文本返回完整的 anatomy map |
| `get_project_rules` | 返回当前项目活跃的 cerebrum 规则 |
| `find_similar_bugs` | 在 buglog 中搜索匹配代码片段的条目 |

通过 `claude mcp add` 注册二进制文件后，这些工具自动可用。

---

## 配置

所有配置通过环境变量或配置文件 `~/.mneme/config.yaml` 完成。

**优先级：环境变量 > 配置文件 > 内置默认值**

配置文件示例 `~/.mneme/config.yaml`：

```yaml
embedding_api_key: sk-xxx
embedding_provider: qwen
embedding_model: text-embedding-v4

db_backend: chromem
chromem_path: ~/.mneme/chromem
```

| 变量 | 默认值 | 说明 |
|---|---|---|
| `EMBEDDING_API_KEY` | （必填） | SiliconFlow 或 Qwen API 密钥 |
| `EMBEDDING_PROVIDER` | `siliconflow` | `siliconflow` 或 `qwen` |
| `EMBEDDING_MODEL` | `BAAI/bge-large-zh-v1.5` | 嵌入模型 ID |
| `DB_BACKEND` | `chromem` | 向量存储后端：`chromem`、`qdrant`、`duckdb`（需 `-tags duckdb`） |
| `DB_PATH` | `~/.mneme/db.duckdb` | DuckDB 文件路径（仅 duckdb 后端） |
| `QDRANT_URL` | `http://localhost:6333` | Qdrant 服务 URL（HTTP 端口；gRPC 6334 自动使用） |
| `QDRANT_COLLECTION` | `mneme` | Qdrant 集合名称 |
| `CHROMEM_PATH` | `~/.mneme/chromem` | chromem-go 持久化目录 |
| `LOG_LEVEL` | `info` | `info` 或 `debug` |

## 后端

### chromem-go（默认）

纯 Go 嵌入式向量存储，无需外部进程，自动持久化到磁盘。

```bash
export DB_BACKEND=chromem
export CHROMEM_PATH="$HOME/.mneme/chromem"
```

### DuckDB

零配置嵌入式数据库，适合单机、中等规模代码库。需要编译时启用 `-tags duckdb`。

```bash
export DB_BACKEND=duckdb
export DB_PATH="$HOME/.mneme/db.duckdb"
```

### Qdrant

通过 gRPC 连接外部 Qdrant 服务，适合大型代码库或共享/多进程部署。首次插入时自动创建集合。

```bash
# 启动 Qdrant（Docker）
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant

# 配置
export DB_BACKEND=qdrant
export QDRANT_URL="http://localhost:6333"
export QDRANT_COLLECTION="mneme"
```

HTTP 端口（6333）会在内部自动转换为 gRPC 端口（6334）。

## MCP 资源

| 资源 | 说明 |
|---|---|
| `codebase://index` | 索引代码库。POST `{"codebase_path": "/path/to/repo"}` |
| `codebase://search` | 语义搜索。POST `{"query": "...", "top_k": 10}` |

## 架构

```
MCP stdio 请求
  → cmd/main.go           (子命令分发)
  → cmd/mcp_server.go     (stdin/stdout JSON-RPC)
  → pkg/context/          (indexer.go: 分割 + 嵌入 + 存储；
                            searcher.go: 嵌入查询 + 检索)
  → pkg/embedding/        (CachedClient 封装 SiliconFlow/Qwen，带 MD5 缓存)
  → pkg/vectordb/         (factory.go 选择 DuckDB / Qdrant / chromem)
  → pkg/splitter/         (Go/Py/JS 按函数边界分割；其他语言 50 行固定分块)

Hook 管道（Claude Code 事件 → cmd/hook_*.go）：
  session-start → 写入记忆行，延迟合并，更新会话
  pre-read      → anatomy 查找，向 Claude 输出文件描述
  pre-write     → cerebrum 规则匹配，buglog 匹配，输出警告
  post-write    → 分类编辑，更新 ledger 和 buglog
  stop          → 递增 stop 计数器

状态文件（项目级，位于 .mneme/）：
  anatomy.md    → 文件索引，由 `scan` 重新生成
  ledger.json   → 累计钩子计数器
  session.json  → 当前会话编辑
  cerebrum.json → 编码规则
  buglog.json   → Bug 历史

全局状态（~/.claude/）：
  mneme-memory.md → 会话历史，按需合并
```

所有跨包契约定义在 `pkg/types.go` 的接口中（`Store`、`EmbeddingProvider`、`Indexer`、`Searcher`、`Splitter`）。

包结构：

- `cmd/` - 二进制入口 + 所有子命令和钩子
- `pkg/` - 核心接口和类型
- `pkg/config/` - 基于环境变量的配置
- `pkg/context/` - 索引和搜索编排
- `pkg/embedding/` - SiliconFlow 和 Qwen 提供者（带缓存）
- `pkg/vectordb/` - DuckDB、Qdrant 和 chromem 后端；工厂选择器
- `pkg/splitter/` - 语言感知的代码分块
- `pkg/mcp/` - MCP 协议服务器
- `pkg/state/` - Ledger、会话、anatomy、记忆和锁原语
- `pkg/scanner/` - 文件遍历和 anatomy 提取器（增量）
- `pkg/consolidator/` - 记忆行合并
- `pkg/hook/` - 钩子事件解析辅助
- `pkg/waste/` - 上下文浪费模式检测

## 开发

```bash
# 构建
go build -o ./bin/mneme ./cmd

# 运行所有测试
go test -v ./...

# 运行单个测试
go test -v ./tests -run TestScanIncrementalSucceeds

# 运行 Qdrant 集成测试（需要运行中的 Qdrant 实例）
QDRANT_URL=http://localhost:6333 go test -v -tags qdrant_integration ./tests

# 调试日志
LOG_LEVEL=debug ./bin/mneme

# 卸载项目钩子
mneme init --uninstall --yes
```

## 许可证

MIT
