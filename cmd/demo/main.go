package main

import (
	"context"
	"log"
	"os"

	"github.com/hongshuo_wang/go-tiny-claw/internal/engine"
	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

type mockProvider struct {
	turn int
}

func (m *mockProvider) Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	m.turn++
	if m.turn == 1 {
		return &schema.Message{
			Role:    schema.RoleAssistant,
			Content: "让我来看看当前目录下有什么文件。",
			ToolCalls: []schema.ToolCall{
				{
					ID:        "call_123",
					Name:      "bash",
					Arguments: []byte(`{"command": "ls -la"}`),
				},
			},
		}, nil
	}
	return &schema.Message{
		Role:    schema.RoleAssistant,
		Content: "我看到了文件列表，里面包含 main.go, 任务完成！",
	}, nil
}

type mockRegistry struct {
}

func (m *mockRegistry) GetAvailableTools() []schema.ToolDefinition {
	return nil
}

func (m *mockRegistry) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	// 直接伪造一段终端输出
	return schema.ToolResult{
		ToolCallID: call.ID,
		Output:     "-rw-r--r-- 1 user group 234 Oct 24 10:00 main.go\n",
		IsError:    false,
	}
}

func main() {
	// 获取当前目录作为 WorkDir的物理边界
	workDir, _ := os.Getwd()

	p := &mockProvider{}
	r := &mockRegistry{}
	// 实例化核心引擎
	agentEngine := engine.NewAgentEngine(p, r, workDir)

	// 发起任务指令
	err := agentEngine.Run(context.Background(), "帮我检查当前目录的文件")
	if err != nil {
		log.Fatalf("引擎崩溃：%v", err)
	}
}
