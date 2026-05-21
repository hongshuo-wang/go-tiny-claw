package main

import (
	"context"
	"log"
	"os"

	"github.com/hongshuo_wang/go-tiny-claw/internal/engine"
	"github.com/hongshuo_wang/go-tiny-claw/internal/provider"
	"github.com/hongshuo_wang/go-tiny-claw/internal/tools"
)

func main() {
	// 获取当前目录作为 WorkDir的物理边界
	workDir, _ := os.Getwd()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("缺少环境变量 OPENAI_API_KEY")
	}

	// 初始化真实的 Provider 大脑
	llmProvider := provider.NewOpenAIProvider("gpt-5.5", "https://api.gapi.cc/v1", apiKey)
	// 注入真实的工具注册中心
	r := tools.NewRegistry()
	// 注册文件读取工具
	readFileTool := tools.NewReadFileTool(workDir)
	r.Registry(readFileTool)
	// 实例化核心引擎
	agentEngine := engine.NewAgentEngine(llmProvider, r, workDir, true)

	// 发起任务指令
	prompt := "查找一下当前工作区的 hello.txt文件，并跟我说一下里面有什么内容？"
	err := agentEngine.Run(context.Background(), prompt)
	if err != nil {
		log.Fatalf("引擎崩溃：%v", err)
	}
}
