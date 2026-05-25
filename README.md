# CS2 Demo AI 分析平台

上传 CS2 demo（.dem），自动解析 + 对比职业选手基线 + LLM 点评。

## 架构（8 模块）

```
HTTP API → Orchestrator → Parser ──┐
                                   ├─→ LLM Analyzer → Report → Storage
            Storage ←───── Pro KB ─┘
```

| 模块 | 路径 | 职责 |
|------|------|------|
| HTTP API | `internal/api` | REST 路由，上传/查询 |
| Orchestrator | `internal/orchestrator` | 异步任务编排，worker pool |
| Storage | `internal/storage` | 文件 + SQLite，demo/report 持久化 |
| Parser | `internal/parser` | .dem → MatchStats |
| Pro KB | `internal/prokb` | 职业选手基线检索 |
| LLM Analyzer | `internal/analyzer` | Claude/OpenAI → 结构化报告 |
| Domain | `internal/domain` | 共享数据结构 |
| 入口 | `cmd/server` | main + 启动 |

## 快速开始

```bash
go mod tidy
export LLM_API_KEY=sk-...   # 兼容 OpenAI 协议的 endpoint
export LLM_BASE_URL=https://api.anthropic.com/v1  # 或其他
go run ./cmd/server

# 另一个终端
curl -F "file=@./testdata/sample.dem" -F "player=Player1" \
     http://localhost:8080/demos
# → {"demo_id":"abc-123","status":"queued"}

curl http://localhost:8080/demos/abc-123/report
```

## 目录

```
cmd/server/              # 主入口
internal/
  api/                   # HTTP handlers
  orchestrator/          # 任务调度
  parser/                # demoinfocs 解析
  prokb/                 # 职业资料库
  analyzer/              # LLM 分析
  storage/               # 存储层
  domain/                # 数据结构
data/                    # 运行时数据（.dem 文件 + sqlite）
```
