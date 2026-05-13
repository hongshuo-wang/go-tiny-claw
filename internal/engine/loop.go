package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/hongshuo_wang/go-tiny-claw/internal/provider"
	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
	"github.com/hongshuo_wang/go-tiny-claw/internal/tools"
)

// AgentEngine 是微型 OS 的核心驱动
type AgentEngine struct {
	provider provider.LLMProvider
	registry tools.Registry

	// WorkDir（工作区）：借鉴 OpenClaw 的理念，Agent 必须有一个明确的物理边界
	WorkDir string
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, workDir string) *AgentEngine {
	return &AgentEngine{
		provider: p,
		registry: r,
		WorkDir:  workDir,
	}
}

// Run 启动 Agent 的生命周期
func (e *AgentEngine) Run(ctx context.Context, userPrompt string) error {
	log.Printf("[Engine] 引擎启动，锁定工作区%s\n", e.WorkDir)

	// 1、初始化会话的 Context(上下文内存)
	// 在真实的场景中，这里会由动态 Prompt 组装起加载 AGENTS.md，目前我们先硬编码
	contextHistory := []schema.Message{
		{
			Role:    schema.RoleSystem,
			Content: "你是 go-tiny-claw，一名专业的编程助手，拥有工作区所有工具的完整使用权限。",
		},
		{
			Role:    schema.RoleUser,
			Content: userPrompt,
		},
	}

	turnCount := 0 // 循环次数

	// 2、The Main Loop: 心跳开始（一个标准的 ReAct）
	for {
		turnCount++
		log.Printf("======= [Turn %d] 开始 =======\n", turnCount)

		// 获取当前挂载的所有工具定义
		availableTools := e.registry.GetAvailableTools()

		// 向大模型发起推理请求（包含 Reasoning）
		log.Println("[Engine] 正在思考（Reasoning）...")
		responseMessage, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("[Engine] 推理失败：%w\n", err)
		}

		// 将模型的响应完整追加到上下文历史中
		contextHistory = append(contextHistory, *responseMessage)

		// 如果模型回复了纯文本，打印出来（这通常是它的思考过程，或是最终结果）
		if responseMessage.Content != "" {
			log.Printf("🤖[Engine] 模型回复：%s\n", responseMessage.Content)
		}

		// 3、退出条件判断
		// 如果模型没有请求任何工具调用，说明它认为任务已经完成，跳出循环。
		if len(responseMessage.ToolCalls) == 0 {
			log.Println("[Engine] 模型没有请求任何工具调用，任务完成。\n")
			break
		}
		// 4、 执行行动（Action）与获取观察结果（Observation）
		log.Printf("[Engine] 模型请求了 %d 个工具调用\n", len(responseMessage.ToolCalls))

		for _, toolCall := range responseMessage.ToolCalls {
			log.Printf("[Engine] 正在调用工具 %s, 参数：%s\n", toolCall.Name, string(toolCall.Arguments))
			// 通过 Registry 路由并执行底层工具
			toolResult := e.registry.Execute(ctx, toolCall)
			if toolResult.IsError {
				log.Printf("[Engine] 工具调用 %s 失败：%s\n", toolCall.Name, toolResult.Output)
			} else {
				log.Printf("[Engine] 工具调用 %s 成功：%s\n", toolCall.Name, toolResult.Output)
			}

			// 将工具执行的观察结果（Observation）封装为 User Message 追加到上下文中
			observationMessage := schema.Message{
				Role:       schema.RoleUser,
				Content:    toolResult.Output,
				ToolCallID: toolCall.ID, // ToolCallID必须携带！这是维系大模型推理链条的关键
			}
			contextHistory = append(contextHistory, observationMessage)
		}
		// 循环回到开头，模型将带着新加入的 Observation 继续它的下一轮思考
	}
	return nil
}
