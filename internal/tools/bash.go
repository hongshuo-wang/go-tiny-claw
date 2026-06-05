package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

type BashTool struct {
	workDir string // 工作区
}

func NewBashTool(workDir string) *BashTool {
	return &BashTool{
		workDir: workDir,
	}
}

func (b *BashTool) Name() string {
	return "bash"
}

func (b *BashTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        "bash",
		Description: "在当前工作区执行任意的 bash 命令。支持链式命令(如 &&)。返回标准输出(stdout)和标准错误(stderr)。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "要执行的 bash 命令，例如 ls -la 或 go test ./..",
				},
			},
			"required": []string{"command"},
		},
	}
}

type BashParams struct {
	Command string `json:"command"`
}

func (b *BashTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var bashParams BashParams
	err := json.Unmarshal(params, &bashParams)
	if err != nil {
		return "", fmt.Errorf("[Bash tool] 参数解析失败：%w", err)
	}

	// 【驾驭底线 1】Time Budgeting（时间预算与超时控制）
	// 给予 bash 命令一个最大执行时间，防止大模型卡死进程（比如说运行了 top 或持续监听的Web 服务）
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, 30*time.Second)
	defer cancelFunc()

	// 在 macOS/Linux下，我们通过将指令包裹在 bash -c中执行，以支持环境变量，管道与(&&)等复杂 Shell 语法
	commandContext := exec.CommandContext(ctx, "bash", "-c", bashParams.Command)

	// 【驾驭底线 2】：绑定执行的工作区目录
	// 确保命令默认在用户指定的 WorkDir 下执行，而不是引擎启动时的绝对路径
	commandContext.Dir = b.workDir
	// 执行并捕获 CombinedOutput（合并 stdout 和 stderr）
	output, err := commandContext.CombinedOutput()
	outputStr := string(output)

	// 如果命令执行超时，返回警告信息让模型知晓
	if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
		return fmt.Sprintf("[Bash tool] 命令执行超时，请检查命令是否正确或执行时间过长。\n%s", outputStr), nil
	}

	// 【驾驭底线 3】：错误原样回传（Self-Correction自愈机制）
	// 当 bash 报错时（err != nil）我们绝对不能返回 Go 的 error 阻断程序
	// 我们必须把 err 和 outputStr 拼接成字符串返回，利用大模型的子就粗能力自己分析报错！
	if err != nil {
		return fmt.Sprintf("[Bash tool] 命令执行错误：%s\n%s", err.Error(), outputStr), nil
	}

	// 如果没有终端输出（比如仅仅执行了 mkdir）,给模型一个明确的执行成功的反馈
	if outputStr == "" {
		return "[Bash tool] 命令执行成功！此命令无终端输出。", nil
	}

	// 【驾驭底线 4】：长度截断保护（防OOM）
	const maxLen = 8000
	if len(outputStr) > maxLen {
		outputStr = outputStr[:maxLen]
		return fmt.Sprintf("[Bash tool] 命令执行成功！终端输出过长，截断至 %d 个字符。\n%s", maxLen, outputStr), nil
	}
	return outputStr, nil
}
