package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hongshuo_wang/go-tiny-claw/internal/engine"
	"github.com/hongshuo_wang/go-tiny-claw/internal/feishu"
	"github.com/hongshuo_wang/go-tiny-claw/internal/provider"
	"github.com/hongshuo_wang/go-tiny-claw/internal/tools"
)

func main() {
	// 获取当前目录作为 WorkDir的物理边界
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("获取当前工作目录失败: %v", err)
	}

	agentEngine := newAgentEngine(workDir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bot := feishu.NewFeishuWebsocketBot(agentEngine)
	log.Println("🚀 飞书 WebSocket 长连接模式启动，按 Ctrl+C 退出...")

	if err := bot.StartWebSocket(ctx); err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			log.Println("收到退出信号，飞书机器人已停止")
			return
		}
		log.Fatalf("❌ WebSocket 连接失败: %v", err)
	}

	log.Println("飞书机器人已停止")
}

func newAgentEngine(workDir string) *engine.AgentEngine {
	apiKey := mustGetEnv("OPENAI_API_KEY")
	modelName := mustGetEnv("OPENAI_MODEL")
	baseUrl := mustGetEnv("OPENAI_BASE_URL")

	log.Printf("初始化 LLM Provider... model:[%s] baseUrl:[%s]\n", modelName, baseUrl)

	llmProvider := provider.NewOpenAIProvider(modelName, baseUrl, apiKey)
	registry := tools.NewRegistry()

	registry.Registry(tools.NewReadFileTool(workDir))
	registry.Registry(tools.NewWriteFileTool(workDir))
	registry.Registry(tools.NewBashTool(workDir))
	registry.Registry(tools.NewEditFileTool(workDir))

	return engine.NewAgentEngine(llmProvider, registry, workDir, false)
}

func mustGetEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("缺少环境变量 %s", key)
	}
	return value
}
