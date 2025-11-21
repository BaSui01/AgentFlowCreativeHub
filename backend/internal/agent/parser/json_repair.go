package parser

import (
	"strings"
)

// RepairJSON 尝试修复常见的 JSON 格式错误
// 1. 移除 Markdown 代码块标记 (```json ... ```)
// 2. 移除首尾空白字符
func RepairJSON(input string) string {
	cleaned := strings.TrimSpace(input)

	// 移除 Markdown 代码块
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 2 {
			// 移除第一行 (```json 或 ```)
			lines = lines[1:]
			// 移除最后一行 (```)
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			cleaned = strings.Join(lines, "\n")
		}
	}

	// 也可以尝试更复杂的正则替换，例如修复尾部逗号等，
	// 但对于大多数 LLM 输出，剥离 Markdown 是最关键的。
	return strings.TrimSpace(cleaned)
}
