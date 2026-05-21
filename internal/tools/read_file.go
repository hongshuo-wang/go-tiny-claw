package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

type ReadFileTool struct {
	// WorkDir 限制工具的工作区，不要越界
	WorkDir string
}

func NewReadFileTool(workDir string) *ReadFileTool {
	return &ReadFileTool{
		WorkDir: workDir,
	}
}

func (r *ReadFileTool) Name() string {
	return "read_file"
}

func (r *ReadFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        "read_file",
		Description: "从系统工作区路径中读取文件内容",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要读取的文件路径，如/cmd/claw/main.go",
				},
			},
			"required": []string{"path"},
		},
	}
}

type readFileArgs struct {
	Path string `json:"path"`
}

func (r *ReadFileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	// 1. 解析参数
	var args readFileArgs
	err := json.Unmarshal(params, &args)
	if err != nil {
		return "", fmt.Errorf("[Read_File Tool]参数解析失败：%w", err)
	}
	// 2. 拼接全路径 （生产环境要做好路径穿越，防止越过工作区越级访问）
	fullPath := filepath.Join(r.WorkDir, filepath.Clean(args.Path))
	log.Printf("[Read_File Tool]正在读取文件：%s", fullPath)
	// 检查一遍，确保路径确实在允许的目录里
	if !strings.HasPrefix(fullPath, r.WorkDir) {
		return "", fmt.Errorf("[Read_File Tool]文件路径越界：%s", args.Path)
	}
	// 3. 打开文件
	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("[Read_File Tool]文件打开失败：%w", err)
	}
	defer file.Close()
	// 4. 读取文件全部内容
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("[Read_File Tool]文件读取失败：%w", err)
	}
	const maxLen = 8000
	if len(fileContent) > maxLen {
		truncatedMsg := fmt.Sprintf("%s\n\n...[由于内容过长，已被系统截断至前 %d 字节]...", string(fileContent[:maxLen]), maxLen)
		return truncatedMsg, nil
	}
	return string(fileContent), nil
}
