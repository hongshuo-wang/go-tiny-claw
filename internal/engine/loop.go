package engine

import (
	"context"
	"fmt"
	"log"
	"sync"

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
	// EnableThinking 是否开启慢思考
	EnableThinking bool
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, workDir string, enableThinking bool) *AgentEngine {
	return &AgentEngine{
		provider:       p,
		registry:       r,
		WorkDir:        workDir,
		EnableThinking: enableThinking,
	}
}

// Run 启动 Agent 的生命周期
func (e *AgentEngine) Run(ctx context.Context, userPrompt string, reporter Reporter) error {
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

		log.Printf("[Engine] 慢思考模式（Thinking Phase）：%t\n", e.EnableThinking)
		log.Printf("================= [Turn %d] 开始 =================\n", turnCount)

		// 获取当前挂载的所有工具定义
		availableTools := e.registry.GetAvailableTools()

		// =======================================================
		// Phase 1：慢思考阶段（Thinking）- 剥夺工具，强制让模型先规划
		// =======================================================
		if e.EnableThinking {
			if reporter != nil {
				// 广播慢思考状态
				reporter.OnThinking(ctx)
			}
			log.Printf("[Engine][Phase1] 剥夺工具访问权，强制进入慢思考与规划阶段...")
			thinkResp, err := e.provider.Generate(ctx, contextHistory, nil)
			if err != nil {
				return fmt.Errorf("Thinking 阶段生成失败：%w\n", err)
			}
			// 如果模型输出了思考过程，我们将其作为 Assistant 消息追加到上下文中
			if thinkResp.Content != "" {
				contextHistory = append(contextHistory, *thinkResp)
				log.Printf("🧠[内部思考 Trace]：\n%s\n", thinkResp.Content)
			}
		}

		// =======================================================
		// Phase 2：行动阶段（Action）- 恢复工具，顺着规划执行
		// =======================================================
		log.Printf("[Engine][Phase2] 挂载工具，等待模型采取行动...")
		// 此时的 contextHistory中已经包含了上一阶段模型自己的 Thinking Trace
		// 模型会顺着自己的逻辑，结合恢复的 availableTools 发起精准的工具调用
		actionResp, err := e.provider.Generate(ctx, contextHistory, availableTools)
		if err != nil {
			return fmt.Errorf("Action 阶段生成失败：%w\n", err)
		}

		// 将模型的响应完整追加到上下文历史中
		contextHistory = append(contextHistory, *actionResp)

		// 如果模型回复了纯文本，打印出来（这通常是它的思考过程，或是最终结果）
		if actionResp.Content != "" {
			log.Printf("🤖[对外回答]：\n%s\n", actionResp.Content)
		}

		// 3、退出条件判断
		// 如果模型没有请求任何工具调用，说明它认为任务已经完成，跳出循环。
		if len(actionResp.ToolCalls) == 0 {
			if actionResp.Content != "" && reporter != nil {
				reporter.OnMessage(ctx, actionResp.Content)
			}
			log.Println("[Engine] 模型没有请求任何工具调用，任务完成。")
			break
		}
		// 4、 执行行动（Action）与获取观察结果（Observation）
		log.Printf("[Engine] 模型并发请求了 %d 个工具调用\n", len(actionResp.ToolCalls))

		// 【驾驭工程】并行调用工具
		// 预分配一个固定长度的切片，用于安全地存放各个并发工具的执行结果
		// 长度与 tool call 的数量完全一致
		observationMsgs := make([]schema.Message, len(actionResp.ToolCalls))

		// 创建一个 WaitGroup，用于等待所有工具调用完成
		var wg sync.WaitGroup

		// 遍历模型请求的所有工具，为每一个工具单独 Fork 出一个 Goroutine
		for i, toolCall := range actionResp.ToolCalls {
			wg.Add(1) // 增加计数器
			// 开启协程。注意：一定要将索引 i 和 toolCall 作为参数传入匿名函数，防止闭包问题
			go func(idx int, call schema.ToolCall) {
				defer wg.Done() // 协程结束时计数器减 1
				if reporter != nil {
					reporter.OnToolCall(ctx, call.Name, string(call.Arguments))
				}
				log.Printf("[Engine Go-%d] 正在并行调用工具 %s, 参数：%s\n", idx, toolCall.Name, string(toolCall.Arguments))
				// 通过 Registry 路由并执行底层工具
				toolResult := e.registry.Execute(ctx, toolCall)
				if reporter != nil {
					// 为了防止大文件
					reporter.OnToolResult(ctx, toolCall.Name, toolResult.Output, toolResult.IsError)
				}
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
				// 线程安全：追加到切片中
				observationMsgs[idx] = observationMessage
			}(i, toolCall)
		}
		// Join 阻塞等待：主循环挂起，知道所有的并发协程全部执行完毕
		wg.Wait()
		log.Println("[Engine] 所有并发模型工具调用完毕，开始聚合观察结果（Observation）...")
		for _, obs := range observationMsgs {
			contextHistory = append(contextHistory, obs)
		}
		// 循环回到开头，模型将带着新加入的 Observation 继续它的下一轮思考
	}
	return nil
}
