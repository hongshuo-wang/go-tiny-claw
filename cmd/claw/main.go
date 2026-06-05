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
	prompt := `我当前目录下有一个 hello.go 文件。 请帮我把里面 
				"TODO: 增加鉴权逻辑" 下面的那个 if 语句，整个替换为： 
				if user == nil { fmt.Println("Forbidden!") return }
				替换时注意代码格式要正确
				`
	err := agentEngine.Run(context.Background(), prompt)
	if err != nil {
		log.Fatalf("引擎崩溃：%v", err)
	}
}
