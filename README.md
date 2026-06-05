# go-tiny-claw

Harness Agent 原型（Go 实现），面向 ReAct + Tool Use + WorkDir 沙箱学习。

## 已实现功能
- CLI 入口：`cmd/claw/main.go`
- OpenAI Provider（`internal/provider/openai.go`）
- 工具 Registry（`internal/tools/registry.go`）+ 4 个工具：read/write/edit_file、bash
- AgentEngine 主循环（`internal/engine/loop.go`）：ReAct、Thinking Phase、WorkDir 边界、上下文历史

## 启动方式
```bash
export OPENAI_API_KEY=sk-xxx
go run ./cmd/claw
```

## 待办清单（来自 todo/）
- **bash_tool.txt**: 支持后台守护进程（TaskManager + nohup 风格，避免 30s 超时杀进程）
- **edit_file.txt**: 替换时自动提取并保留目标行基础缩进
- **loop.txt**: 工具调用并行执行（goroutine + WaitGroup + 顺序回填 Observation）
- **registry.txt**: 增加自纠错重试防护机制（防止盲目重试失控）
- **stream_output.txt**: Provider 流式响应改造（SSE + channel），Main Loop 边收边打
