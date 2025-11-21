package parser

import (
	"encoding/json"
	"fmt"
)

// Parser 输出解析器接口
type Parser interface {
	// Parse 解析字符串输出
	Parse(text string) (any, error)
	// FormatInstructions 获取格式化指令（注入到 Prompt 中）
	FormatInstructions() string
}

// JSONParser JSON 解析器
type JSONParser struct {
	Schema any // 预期的 JSON 结构 (用于生成说明，可选)
}

// NewJSONParser 创建 JSON 解析器
func NewJSONParser(schema any) *JSONParser {
	return &JSONParser{
		Schema: schema,
	}
}

func (p *JSONParser) Parse(text string) (any, error) {
	// 1. 尝试修复 JSON
	cleaned := RepairJSON(text)

	// 2. 解析
	var result any
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	return result, nil
}

func (p *JSONParser) FormatInstructions() string {
	base := "输出必须是有效的 JSON 格式。"
	if p.Schema != nil {
		schemaBytes, _ := json.MarshalIndent(p.Schema, "", "  ")
		base += fmt.Sprintf("\n请遵循以下 JSON 结构示例：\n```json\n%s\n```", string(schemaBytes))
	}
	return base
}

