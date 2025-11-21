package executor

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"backend/internal/agent/runtime"
)

// AgentSwitcher Agent 智能切换器
type AgentSwitcher struct {
	config *AgentSwitchConfig
}

// NewAgentSwitcher 创建 Agent 切换器
func NewAgentSwitcher(config *AgentSwitchConfig) *AgentSwitcher {
	if config == nil {
		config = &AgentSwitchConfig{
			Mode:  "static",
			Rules: []AgentSwitchRule{},
		}
	}
	
	// 按优先级排序规则
	sort.Slice(config.Rules, func(i, j int) bool {
		return config.Rules[i].Priority < config.Rules[j].Priority
	})
	
	return &AgentSwitcher{
		config: config,
	}
}

// DetermineNextAgent 决定下一个 Agent
func (s *AgentSwitcher) DetermineNextAgent(ctx context.Context, 
	currentStep string, 
	currentResult *runtime.AgentResult) (string, error) {
	
	// 静态模式：不切换，由工作流定义决定
	if s.config.Mode == "static" {
		return "", nil
	}
	
	// 动态模式：根据规则决定
	for _, rule := range s.config.Rules {
		matched, err := s.evaluateCondition(rule.Condition, currentResult)
		if err != nil {
			continue // 跳过无法评估的规则
		}
		
		if matched {
			return rule.NextAgent, nil
		}
	}
	
	// 没有匹配的规则，不切换
	return "", nil
}

// evaluateCondition 评估条件表达式
func (s *AgentSwitcher) evaluateCondition(expression string, result *runtime.AgentResult) (bool, error) {
	if expression == "" {
		return false, fmt.Errorf("空条件表达式")
	}
	
	// 简化版本：支持基本的比较表达式
	// 格式：field operator value
	// 例如：quality_score < 80
	
	parts := strings.Fields(expression)
	if len(parts) < 3 {
		return false, fmt.Errorf("无效的条件表达式: %s", expression)
	}
	
	field := parts[0]
	operator := parts[1]
	valueStr := parts[2]
	
	// 从结果中获取字段值
	var fieldValue float64
	switch field {
	case "quality_score":
		if result.Metadata != nil {
			if score, ok := result.Metadata["quality_score"].(float64); ok {
				fieldValue = score
			}
		}
	case "confidence":
		if result.Metadata != nil {
			if conf, ok := result.Metadata["confidence"].(float64); ok {
				fieldValue = conf
			}
		}
	case "cost":
		fieldValue = result.Cost
	default:
		return false, fmt.Errorf("不支持的字段: %s", field)
	}
	
	// 解析比较值
	compareValue, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return false, fmt.Errorf("无效的比较值: %s", valueStr)
	}
	
	// 执行比较
	switch operator {
	case "<":
		return fieldValue < compareValue, nil
	case "<=":
		return fieldValue <= compareValue, nil
	case ">":
		return fieldValue > compareValue, nil
	case ">=":
		return fieldValue >= compareValue, nil
	case "==":
		return fieldValue == compareValue, nil
	case "!=":
		return fieldValue != compareValue, nil
	default:
		return false, fmt.Errorf("不支持的操作符: %s", operator)
	}
}

// ShouldRetry 判断是否应该重试
func (s *AgentSwitcher) ShouldRetry(result *runtime.AgentResult, retryConfig *RetryConfig, currentRetry int) bool {
	if retryConfig == nil {
		return false
	}
	
	// 检查是否超过最大重试次数
	if currentRetry >= retryConfig.MaxRetries {
		return false
	}
	
	// 检查是否失败
	if result.Status == "failed" || result.Error != "" {
		return true
	}
	
	return false
}

// GetRetryDelay 获取重试延迟时间（秒）
func (s *AgentSwitcher) GetRetryDelay(retryConfig *RetryConfig, currentRetry int) int {
	if retryConfig == nil {
		return 0
	}
	
	switch retryConfig.Backoff {
	case "exponential":
		// 指数退避：delay * 2^retry
		delay := retryConfig.Delay
		for i := 0; i < currentRetry; i++ {
			delay *= 2
		}
		return delay
	default: // "fixed"
		return retryConfig.Delay
	}
}
