package main

import (
	"context"
	"log"
	"os"

	"github.com/hongshuo_wang/go-tiny-claw/internal/engine"
	"github.com/hongshuo_wang/go-tiny-claw/internal/provider"
	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

type mockRegistry struct {
}

func (m *mockRegistry) GetAvailableTools() []schema.ToolDefinition {
	// 定义一个获取天气情况的 tool definition
	return []schema.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "获取指定城市的当前天气情况",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{
						"type":        "string",
						"description": "城市名称",
					},
				},
				"required": []string{"city"},
			},
		},
	}
}

func (m *mockRegistry) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	// 直接伪造一段终端输出
	log.Printf(" -> [Mock 工具执行] 获取 %s 的天气中...\n", call.Name)
	return schema.ToolResult{
		ToolCallID: call.ID,
		Output:     "API 返回：今天是晴天，气温 25度",
		IsError:    false,
	}
}

func main() {
	// 获取当前目录作为 WorkDir的物理边界
	workDir, _ := os.Getwd()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("缺少环境变量 OPENAI_API_KEY")
	}

	// 初始化真实的 Provider 大脑
	llmProvider := provider.NewOpenAIProvider("gpt-5.5", "https://api.gapi.cc/v1", apiKey)
	// 注入伪造的工具注册中心
	r := &mockRegistry{}
	// 实例化核心引擎
	agentEngine := engine.NewAgentEngine(llmProvider, r, workDir, true)

	// 发起任务指令
	prompt := "我想去北京跑步，帮我查查天气适合吗？"
	err := agentEngine.Run(context.Background(), prompt)
	if err != nil {
		log.Fatalf("引擎崩溃：%v", err)
	}
}
