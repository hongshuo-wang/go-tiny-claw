package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

// Registry 定义了工具的注册与分发执行接口
type Registry interface {
	Registry(tool BaseTool)
	// GetAvailableTools 返回当前系统挂载的所有可用工具的 Schema
	GetAvailableTools() []schema.ToolDefinition
	// Execute 实际执行模型请求的工具，并返回结果
	Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult
}

// BaseTool 是所有具体工具必须实现的通用接口
type BaseTool interface {
	// Name 返回工具的全局唯一名称（大模型通过这个名字调用它）
	Name() string
	// Definition 描述工具，返回用于提交给大模型的工具元信息和参数 JSON Schema
	Definition() schema.ToolDefinition
	// Execute 接收大模型吐出的 JSON 参数，执行具体业务逻辑
	Execute(ctx context.Context, params json.RawMessage) (string, error)
}

// RegistryImpl 默认实现
type RegistryImpl struct {
	tools map[string]BaseTool // 封装一个 map 用于存储所有工具，查找O(1)
}

func NewRegistry() *RegistryImpl {
	return &RegistryImpl{
		tools: make(map[string]BaseTool),
	}
}

func (r *RegistryImpl) Registry(tool BaseTool) {
	toolName := tool.Name()
	// 把工具注册到 map 中，如果重复了，就告警，只注册第一个工具
	if _, exist := r.tools[toolName]; exist {
		log.Printf("[Registry] 工具 %s 重复注册，已忽略", toolName)
		return
	}
	// 把工具注册到 map 中
	r.tools[toolName] = tool
}

func (r *RegistryImpl) GetAvailableTools() []schema.ToolDefinition {
	var toolDefinitions []schema.ToolDefinition
	for _, tool := range r.tools {
		toolDefinitions = append(toolDefinitions, tool.Definition())
	}
	return toolDefinitions
}

func (r *RegistryImpl) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	// 1. 路由查找：如果在注册表中找不到该工具，这是模型产生了幻觉，直接向模型抛出错误
	tool, exists := r.tools[call.Name]
	if !exists {
		errMsg := fmt.Sprintf("Error: 系统中不存在名为 '%s' 的工具。", call.Name)
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     errMsg,
			IsError:    true, // 标记为错误，模型看到后会尝试纠正
		}
	}
	// 2. 执行工具逻辑：将原始的 JSON 字节流直接丢给具体工具
	output, err := tool.Execute(ctx, call.Arguments)
	// 3. 封装结果：将执行结果或底层物理错误封装后返回给 Main Loop
	if err != nil {
		errMsg := fmt.Sprintf("Error executing %s: %v", call.Name, err)
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     errMsg,
			IsError:    true,
		}
	}
	return schema.ToolResult{
		ToolCallID: call.ID,
		Output:     output,
		IsError:    false,
	}
}
