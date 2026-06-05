package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hongshuo_wang/go-tiny-claw/internal/schema"
)

type EditFileTool struct {
	workDir string // 工作目录
}

func NewEditFileTool(workDir string) *EditFileTool {
	return &EditFileTool{
		workDir: workDir,
	}
}

func (e *EditFileTool) Name() string {
	return "edit_file"
}

func (e *EditFileTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        e.Name(),
		Description: "对现有文件进行局部的字符串替换。这边重写整个文件更安全、更快速。请提供足够的 old_text上下文以确保匹配的唯一性",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要修改的文件路径",
				},
				"old_text": map[string]any{
					"type":        "string",
					"description": "文件中原有的要替换的文本。必须包含足够的上下文（建议上下各多包含几行）",
				},
				"new_text": map[string]any{
					"type":        "string",
					"description": "要替换成的新文本",
				},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
	}
}

type EditFileParams struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (e *EditFileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var editFileParams EditFileParams
	err := json.Unmarshal(params, &editFileParams)
	if err != nil {
		return "", fmt.Errorf("[Edit File Tool] 参数解析错误: %v", err)
	}
	fullPath := filepath.Join(e.workDir, editFileParams.Path)
	// 1、读取原始文件，确认文件存在
	fileBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("[Edit File Tool] 读取文件错误: %v", err)
	}
	originContent := string(fileBytes)
	// 2、调用多级模糊替换算法
	newContent, err := fuzzyReplace(originContent, editFileParams.OldText, editFileParams.NewText)
	if err != nil {
		// 【驾驭哲学】将具体的报错原因原样返回（如匹配到多处），让大模型自行纠正
		return "", fmt.Errorf("[Edit File Tool] 替换错误: %v", err)
	}
	// 3、将新内容安全地写回文件中
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("[Edit File Tool] 写入文件错误: %v", err)
	}
	return fmt.Sprintf("[Edit File Tool] 成功修改文件：%s", editFileParams.Path), nil
}

// fuzzyReplace 多级模糊替换算法
func fuzzyReplace(originContent string, oldText string, newText string) (string, error) {
	// L1: 精确匹配
	count := strings.Count(originContent, oldText)
	if count == 1 {
		return strings.Replace(originContent, oldText, newText, 1), nil
	}
	if count > 1 {
		return "", fmt.Errorf("[Edit File Tool] 匹配到 %d 个结果，请提供更准确的上下文代码以确保唯一性", count)
	}
	// L2：换行符归一化（统一将 \r\n 换成 \n）
	normalizedContent := strings.ReplaceAll(originContent, "\r\n", "\n")
	normalizedOld := strings.ReplaceAll(oldText, "\r\n", "\n")
	count = strings.Count(normalizedContent, normalizedOld)
	if count == 1 {
		return strings.Replace(normalizedContent, normalizedOld, newText, 1), nil
	}
	// L3: Trim Space 匹配（忽略首尾的空行和空格）
	trimmedOld := strings.TrimSpace(normalizedOld)
	if trimmedOld != "" {
		count = strings.Count(normalizedContent, trimmedOld)
		if count == 1 {
			// 注意：这里替换时，我们只能替换被 Trim 后的部分，不能直接用 newText 破坏原本的缩进
			// 为了保持本项目代码不过于冗长复杂，当触发 L3/L4 时，如果 newText 没有带有正确的缩进，
			// 可能会导致替换后代码格式不美观。但这总比直接报错让 Agent 死循环要好。
			return strings.Replace(normalizedContent, trimmedOld, newText, 1), nil
		}
	}
	// L4：逐行去缩进匹配（最强里的容错：消除大模型遗漏缩进的幻觉）
	return lineByLineReplace(normalizedContent, normalizedOld, newText)
}

func lineByLineReplace(normalizedContent string, normalizedOld string, newText string) (string, error) {
	contentLines := strings.Split(normalizedContent, "\n") // 原始逐行拆分文案
	oldLines := strings.Split(normalizedOld, "\n")         // 目标逐行拆分文案

	if len(oldLines) == 0 || len(oldLines) > len(contentLines) {
		return "", fmt.Errorf("[Edit File Tool] 找不到该代码片段，请提供更准确的上下文代码以确保唯一性")
	}

	// 清理 AI 生成的 oldLines 的每行首尾空白
	for i := range oldLines {
		oldLines[i] = strings.TrimSpace(oldLines[i])
	}
	// 滑动窗口在原始文件中寻找匹配块
	matchCount := 0
	matchStartIndex := -1
	matchEndIndex := -1
	for i := 0; i < len(contentLines)-len(oldLines); i++ {
		isMatch := true
		for j := 0; j < len(oldLines); j++ {
			if strings.TrimSpace(contentLines[i+j]) != oldLines[j] {
				isMatch = false
				break
			}
		}
		if isMatch {
			// 找到一次匹配块，记录一次
			matchCount++
			matchStartIndex = i
			matchEndIndex = i + len(oldLines)
		}
	}
	if matchCount == 0 {
		return "", fmt.Errorf("[Edit File Tool] 找不到该代码片段，请大模型先调用 read_file工具仔细阅读代码后重新评估")
	}
	if matchCount > 1 {
		return "", fmt.Errorf("[Edit File Tool] 匹配到 %d 个相似代码结果，请提供更准确的上下文代码以确保唯一性", matchCount)
	}
	// 走到这里表示正好 1 处匹配代码
	// 执行替换：将匹配到的原始行范围替换为 newText 拆分后的行
	var newContentLines []string
	newContentLines = append(newContentLines, contentLines[:matchStartIndex]...)
	newContentLines = append(newContentLines, strings.Split(newText, "\n")...)
	newContentLines = append(newContentLines, contentLines[matchEndIndex:]...)
	return strings.Join(newContentLines, "\n"), nil
}
