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
	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		log.Fatal("缺少环境变量 OPENAI_MODEL")
	}
	baseUrl := os.Getenv("OPENAI_BASE_URL")
	if baseUrl == "" {
		log.Fatal("缺少环境变量 OPENAI_BASE_URL")
	}

	log.Printf("初始化 LLM Provider... model:[%s]  baseUrl:[%s]\n", modelName, baseUrl)

	// 初始化真实的 Provider 大脑
	llmProvider := provider.NewOpenAIProvider(modelName, baseUrl, apiKey)
	// 注入真实的工具注册中心
	r := tools.NewRegistry()
	// 注册文件读取工具
	readFileTool := tools.NewReadFileTool(workDir)
	writeFileTool := tools.NewWriteFileTool(workDir)
	bashTool := tools.NewBashTool(workDir)
	editFileTool := tools.NewEditFileTool(workDir)
	r.Registry(readFileTool)
	r.Registry(writeFileTool)
	r.Registry(bashTool)
	r.Registry(editFileTool)
	// 实例化核心引擎
	agentEngine := engine.NewAgentEngine(llmProvider, r, workDir, false)

	// 发起任务指令
	prompt := `我当前目录下有 a.txt, b.txt, c.txt 三个文件。 
				为了节省时间，请你同时一次性读取这三个文件，并将它们的内容综合起来，告诉我它们分别记录了什么领域的信息
				`
	err := agentEngine.Run(context.Background(), prompt)
	if err != nil {
		log.Fatalf("引擎崩溃：%v", err)
	}
}
