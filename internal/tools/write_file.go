package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

// WriteFileTool 读取、新建文件工具
type WriteFileTool struct {
	WorkDir string // 限制工具工作区，不要越界
}

func (w *WriteFileTool) Name() string {
	return "write_file"
}

func (w *WriteFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        w.Name(),
		Description: "创建或覆盖写入一个文件，如果目录不存在会自动创建。请提供相对于工作区的相对路径。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要写入的相对于工作区的文件路径，如 src/main.go",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "要写入的内容",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *WriteFileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var args writeFileArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return "", fmt.Errorf("参数解析错误: %v", err)
	}
	path := args.Path
	content := args.Content
	// 拼接工作区路径
	joinedPath := filepath.Join(w.WorkDir, path)
	// 如果涉及到父文件夹，需要自动创建好文件夹
	dir := filepath.Dir(joinedPath)
	if err := os.MkdirAll(dir, 0577); err != nil {
		return "", fmt.Errorf("创建文件夹失败: %v", err)
	}
	// 写入文件
	if err := os.WriteFile(joinedPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}
	return fmt.Sprintf("[Write File Tool]成功写入文件：%s", joinedPath), nil
}

func NewWriteFileTool(workDir string) *WriteFileTool {
	return &WriteFileTool{
		WorkDir: workDir,
	}
}
