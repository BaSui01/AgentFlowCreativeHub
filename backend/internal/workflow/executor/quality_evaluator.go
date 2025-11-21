package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"backend/internal/agent/runtime"
)

// QualityEvaluator 质量评估器
type QualityEvaluator struct {
	agentRegistry *runtime.Registry
}

// NewQualityEvaluator 创建质量评估器
func NewQualityEvaluator(agentRegistry *runtime.Registry) *QualityEvaluator {
	return &QualityEvaluator{
		agentRegistry: agentRegistry,
	}
}

// EvaluateQuality 评估输出质量
func (e *QualityEvaluator) EvaluateQuality(ctx context.Context, 
	output string, 
	config *QualityCheckConfig) (float64, error) {
	
	if config == nil || !config.Enabled {
		return 100.0, nil // 未启用质量检查，返回满分
	}
	
	// 使用 Analyzer Agent 进行质量评估
	// 注意：需要提供 tenantID，这里暂时使用空字符串（应该从上下文获取）
	analyzerAgent, err := e.agentRegistry.GetAgentByType(ctx, "", "analyzer")
	if err != nil {
		return 0, fmt.Errorf("获取 Analyzer Agent 失败: %w", err)
	}
	
	// 构建评估输入
	input := &runtime.AgentInput{
		Content: fmt.Sprintf("请评估以下内容的质量（0-100分）：\n\n%s", output),
		Variables: map[string]any{
			"task": "quality_evaluation",
		},
	}
	
	// 执行评估
	result, err := analyzerAgent.Execute(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("执行质量评估失败: %w", err)
	}
	
	// 解析评分
	score := e.parseScore(result.Output)
	
	return score, nil
}

// parseScore 从输出中解析分数
func (e *QualityEvaluator) parseScore(output string) float64 {
	// 尝试解析 JSON 格式
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err == nil {
		if score, ok := data["score"].(float64); ok {
			return score
		}
		if score, ok := data["quality_score"].(float64); ok {
			return score
		}
	}
	
	// 尝试查找分数关键词
	output = strings.ToLower(output)
	
	// 查找"分数："、"score:"等模式
	patterns := []string{
		"分数：",
		"分数:",
		"score:",
		"score：",
		"评分：",
		"评分:",
	}
	
	for _, pattern := range patterns {
		if idx := strings.Index(output, pattern); idx >= 0 {
			// 提取分数后的数字
			rest := output[idx+len(pattern):]
			var score float64
			if _, err := fmt.Sscanf(rest, "%f", &score); err == nil {
				return score
			}
		}
	}
	
	// 默认返回中等分数
	return 70.0
}

// NeedsRewrite 判断是否需要重写
func (e *QualityEvaluator) NeedsRewrite(score float64, config *QualityCheckConfig) bool {
	if config == nil || !config.Enabled {
		return false
	}
	
	return score < config.MinScore && config.RetryOnFail
}
