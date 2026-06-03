# CS2 Demo AI 分析平台

一个本地运行的 CS2 demo 复盘工具。它支持上传 `.dem` 或 5E 下载的 `.zip`，自动解析 demo 内玩家列表，选择目标玩家后生成数据复盘和离线/LLM 分析报告。

## 功能

- 上传 `.dem` 或 `.zip`，服务端会自动抽取 demo 文件。
- 选择文件后自动识别 demo 内真实玩家昵称，避免平台 ID 和 demo 昵称不一致。
- 异步解析击杀、回合、经济、道具、移动、残局等数据。
- 默认可离线生成规则报告；配置 LLM key 后可启用模型点评。
- Web 页面默认运行在 `http://127.0.0.1:8080/`。

## 快速开始

需要 Go 1.24+。

```bash
git clone https://github.com/simonzhw32-cyber/cs2demo.git
cd cs2demo
go mod download
go run ./cmd/server
```

打开浏览器访问：

```text
http://127.0.0.1:8080/
```

使用流程：

1. 选择 `.dem` 或 `.zip` 文件。
2. 等待页面自动解析玩家列表。
3. 从下拉框选择要分析的玩家。
4. 点击“开跑”，等待任务完成。

## 可选 LLM 配置

不配置 API key 也能运行，系统会使用离线规则报告。需要 LLM 点评时设置环境变量：

```bash
# Anthropic
export LLM_PROVIDER=anthropic
export ANTHROPIC_API_KEY=your-key

# OpenAI
export LLM_PROVIDER=openai
export OPENAI_API_KEY=your-key
```

常用配置：

```bash
export HTTP_ADDR=:8080
export DATA_DIR=./data
export SQLITE_PATH=./data/cs2demo.db
export WORKER_COUNT=2
export MAX_UPLOAD_MB=1024
```

## 项目结构

```text
cmd/server/          HTTP 服务入口
cmd/promptdump/      导出已处理 demo 的 analyzer prompt
internal/api/        REST 路由和上传接口
internal/orchestrator/ 异步任务调度
internal/parser/     CS2 .dem 解析
internal/analyzer/   离线规则和 LLM 报告生成
internal/storage/    SQLite 与 demo 文件存储
internal/prokb/      职业基线数据
internal/domain/     共享数据类型
web/                 静态前端页面
data/                运行时数据，默认不提交
```

## 开发命令

```bash
go test ./...
go build ./cmd/server
go run ./cmd/server
go mod tidy
```

## 注意事项

- 不要提交 demo 文件、SQLite 数据库、API key、构建出的 `.exe`。
- 5E 的 zip 有时会被 Windows 报 “archive corrupt”，本项目会尝试从本地 ZIP 头中恢复 `.dem`。
- 如果别人 clone 后打不开页面，先确认服务端正在运行，并访问 `http://127.0.0.1:8080/`。
