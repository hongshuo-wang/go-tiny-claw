package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type OpenAIProvider struct {
	client  openai.Client // 值类型，非指针
	model   string
	baseURL string // 兼容端点
	apiKey  string // apiKey
}

func NewOpenAIProvider(model string, baseURL string, apiKey string) *OpenAIProvider {
	// todo 替换 baseURL，apiKey 从环境变量导入
	return &OpenAIProvider{
		client: openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL)),
		model:  model,
	}
}

func (p *OpenAIProvider) Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	// 定义一个 openai 的消息体格式列表
	var openaiMsgs []openai.ChatCompletionMessageParamUnion
	// 1. 把历史对话的自定义的消息体抓换成 openai sdk 识别的格式，此处准备的都是历史消息列表
	for _, message := range messages {
		switch message.Role {
		case schema.RoleSystem:
			openaiMsgs = append(openaiMsgs, openai.SystemMessage(message.Content))
		case schema.RoleUser:
			// 翻译用户输入
			// 这里有两种情况，一种是调用了工具，一种是响应了内容，工具没有单独抽象一个 Message 类型，复用了 UserMessage
			if message.ToolCallID != "" {
				openaiMsgs = append(openaiMsgs, openai.ToolMessage(message.Content, message.ToolCallID))
			} else {
				openaiMsgs = append(openaiMsgs, openai.UserMessage(message.Content))
			}
		case schema.RoleAssistant:
			// 翻译模型输出
			astParam := openai.ChatCompletionAssistantMessageParam{}

			if message.Content != "" {
				astParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(message.Content),
				}
			}
			// 如果历史包含 ToolCalls，必须原样放回，以维系大模型的逻辑链
			if len(message.ToolCalls) > 0 {
				var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
				for _, tc := range message.ToolCalls {
					// OfFunction 对应 GetFunction()，字段类型严格要求为指针
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID:   tc.ID,
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(tc.Arguments),
							},
						},
					})
				}
				astParam.ToolCalls = toolCalls
			}
			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &astParam,
			},
			)
		}
	}

	// 2. 翻译工具定义，准备携带工具定义 schema 作为请求体发送
	var openaiTools []openai.ChatCompletionToolUnionParam
	for _, toolDef := range availableTools {
		var params shared.FunctionParameters
		// 为了兼容直接传 map 和传 struct 两种类型的 tools，这里做一个兼容
		// 先判断是否是直接传了 map，如果是，直接强转成 FunctionParameters
		if m, ok := toolDef.InputSchema.(map[string]any); ok {
			params = m
		} else {
			// 用 json 序列化将 struct 解析一遍
			b, _ := json.Marshal(toolDef.InputSchema)
			_ = json.Unmarshal(b, &params)
		}
		openaiTools = append(openaiTools, openai.ChatCompletionFunctionTool(
			shared.FunctionDefinitionParam{
				Name:        toolDef.Name,
				Description: openai.String(toolDef.Description),
				Parameters:  params,
			},
		))
	}

	// 3. 构建请求并发送
	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: openaiMsgs,
	}
	// 慢思考模式支撑：仅当 availableTools 存在时才挂载 Tools
	if len(availableTools) > 0 {
		params.Tools = openaiTools
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API [%s] 请求失败：%w", p.model, err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API [%s] 返回空 Choices", p.model)
	}

	// 4. 将 API Response 反向翻译为内部 schema.Message格式
	choice := resp.Choices[0].Message
	log.Printf("OpenAI API [%s] 响应：%s", p.model, choice.RawJSON())
	resultMsg := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: choice.Content,
	}

	for _, tc := range choice.ToolCalls {
		if tc.Type == "function" {
			resultMsg.ToolCalls = append(resultMsg.ToolCalls, schema.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: []byte(tc.Function.Arguments), // 提取 JSON 字符串字节
			})
		}
	}

	return resultMsg, nil
}
