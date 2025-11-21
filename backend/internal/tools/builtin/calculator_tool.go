package builtin

import (
	"context"
	"fmt"
	"math"

	"backend/internal/tools"
)

// CalculatorTool 计算器工具
type CalculatorTool struct{}

// NewCalculatorTool 创建计算器工具
func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

// Execute 执行计算
func (t *CalculatorTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	operation, ok := input["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation 参数类型错误")
	}
	
	switch operation {
	case "add":
		a, _ := input["a"].(float64)
		b, _ := input["b"].(float64)
		return map[string]any{"result": a + b}, nil
		
	case "subtract":
		a, _ := input["a"].(float64)
		b, _ := input["b"].(float64)
		return map[string]any{"result": a - b}, nil
		
	case "multiply":
		a, _ := input["a"].(float64)
		b, _ := input["b"].(float64)
		return map[string]any{"result": a * b}, nil
		
	case "divide":
		a, _ := input["a"].(float64)
		b, _ := input["b"].(float64)
		if b == 0 {
			return nil, fmt.Errorf("除数不能为 0")
		}
		return map[string]any{"result": a / b}, nil
		
	case "power":
		base, _ := input["base"].(float64)
		exponent, _ := input["exponent"].(float64)
		return map[string]any{"result": math.Pow(base, exponent)}, nil
		
	case "sqrt":
		number, _ := input["number"].(float64)
		if number < 0 {
			return nil, fmt.Errorf("不能对负数开方")
		}
		return map[string]any{"result": math.Sqrt(number)}, nil
		
	default:
		return nil, fmt.Errorf("不支持的操作: %s", operation)
	}
}

// Validate 验证输入
func (t *CalculatorTool) Validate(input map[string]any) error {
	operation, ok := input["operation"].(string)
	if !ok {
		return fmt.Errorf("缺少 operation 参数")
	}
	
	switch operation {
	case "add", "subtract", "multiply", "divide":
		if _, ok := input["a"]; !ok {
			return fmt.Errorf("缺少参数 a")
		}
		if _, ok := input["b"]; !ok {
			return fmt.Errorf("缺少参数 b")
		}
	case "power":
		if _, ok := input["base"]; !ok {
			return fmt.Errorf("缺少参数 base")
		}
		if _, ok := input["exponent"]; !ok {
			return fmt.Errorf("缺少参数 exponent")
		}
	case "sqrt":
		if _, ok := input["number"]; !ok {
			return fmt.Errorf("缺少参数 number")
		}
	default:
		return fmt.Errorf("不支持的操作: %s", operation)
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *CalculatorTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "calculator",
		DisplayName: "计算器",
		Description: "执行基本数学计算（加减乘除、乘方、开方）",
		Category:    "calculation",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{
					"type": "string",
					"enum": []string{"add", "subtract", "multiply", "divide", "power", "sqrt"},
					"description": "计算操作类型",
				},
				"a": map[string]any{
					"type":        "number",
					"description": "第一个操作数（加减乘除）",
				},
				"b": map[string]any{
					"type":        "number",
					"description": "第二个操作数（加减乘除）",
				},
				"base": map[string]any{
					"type":        "number",
					"description": "底数（power 操作）",
				},
				"exponent": map[string]any{
					"type":        "number",
					"description": "指数（power 操作）",
				},
				"number": map[string]any{
					"type":        "number",
					"description": "待开方的数（sqrt 操作）",
				},
			},
			"required": []string{"operation"},
		},
		Timeout:    5,
		Status:     "active",
		RequireAuth: false,
	}
}
