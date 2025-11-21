package executor

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/Knetic/govaluate"
)

// ConditionExecutor 条件分支执行器
// 根据条件表达式的评估结果，选择不同的执行路径
type ConditionExecutor struct {
	taskExecutor TaskExecutor
}

// NewConditionExecutor 创建条件执行器
func NewConditionExecutor(taskExecutor TaskExecutor) *ConditionExecutor {
	return &ConditionExecutor{
		taskExecutor: taskExecutor,
	}
}

// EvaluateCondition 评估条件表达式
// 支持的语法：
// - step_1.output.quality_score > 80
// - step_2.output.status == "success"
// - step_1.output.word_count >= 1000
// - step_3.output.error != nil
func (e *ConditionExecutor) EvaluateCondition(expr string, execCtx *ExecutionContext) (bool, error) {
	if expr == "" {
		return false, fmt.Errorf("条件表达式不能为空")
	}

	expr = strings.TrimSpace(expr)

	if fnResult, handled, err := e.evaluateFunction(expr, execCtx); handled {
		return fnResult, err
	}

	// 提前替换 {{ var }} 形式的变量为占位符，避免 govaluate 解析出错
	placeholderMap := make(map[string]string)
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	processedExpr := re.ReplaceAllStringFunc(expr, func(match string) string {
		content := strings.TrimSpace(match[2 : len(match)-2])
		placeholder := fmt.Sprintf("var%d", len(placeholderMap))
		placeholderMap[placeholder] = content
		return placeholder
	})

	// 创建表达式
	expression, err := govaluate.NewEvaluableExpression(processedExpr)
	if err != nil {
		return false, fmt.Errorf("解析表达式失败: %w", err)
	}

	parameters := make(map[string]interface{})

	// 先填充占位符映射
	for placeholder, varName := range placeholderMap {
		val, err := e.getVariableValue(varName, execCtx)
		if err != nil {
			parameters[placeholder] = nil
			continue
		}
		parameters[placeholder] = val
	}

	// 处理剩余变量
	for _, v := range expression.Vars() {
		if _, exists := parameters[v]; exists {
			continue
		}
		val, err := e.getVariableValue(v, execCtx)
		if err != nil {
			parameters[v] = nil
			continue
		}
		parameters[v] = val
	}

	result, err := expression.Evaluate(parameters)
	if err != nil {
		return false, fmt.Errorf("评估表达式失败: %w", err)
	}

	if boolResult, ok := result.(bool); ok {
		return boolResult, nil
	}

	return false, fmt.Errorf("表达式结果不是布尔值: %v", result)
}

// resolveValue 解析值（变量或字面量）
func (e *ConditionExecutor) resolveValue(value string, execCtx *ExecutionContext) (any, error) {
	if strings.HasPrefix(value, "len(") && strings.HasSuffix(value, ")") {
		inner := strings.TrimSpace(value[4 : len(value)-1])
		val, err := e.resolveValue(inner, execCtx)
		if err != nil {
			return nil, err
		}
		return float64(lengthOf(val)), nil
	}

	// 检查是否是变量引用 {{...}}
	if strings.HasPrefix(value, "{{") && strings.HasSuffix(value, "}}") {
		varName := strings.TrimSpace(value[2 : len(value)-2])
		return e.getVariableValue(varName, execCtx)
	}

	// 检查是否是字符串字面量 "..."
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return value[1 : len(value)-1], nil
	}

	// 检查是否是数字
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num, nil
	}

	// 检查是否是布尔值
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}

	// 检查是否是 null
	if value == "null" {
		return nil, nil
	}

	// 其他情况，作为字符串处理
	return value, nil
}

// getVariableValue 从执行上下文中获取变量值
// 支持点号访问嵌套字段：step_1.output.content
func (e *ConditionExecutor) getVariableValue(varName string, execCtx *ExecutionContext) (any, error) {
	parts := strings.Split(varName, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("无效的变量名: %s", varName)
	}

	// 从上下文数据中获取值
	var current any = execCtx.Data

	for i, part := range parts {
		if current == nil {
			return nil, fmt.Errorf("变量 %s 在路径 %s 处为 null", varName, strings.Join(parts[:i], "."))
		}

		// 使用反射访问字段
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("变量 %s 不存在（路径: %s）", varName, strings.Join(parts[:i+1], "."))
			}
			current = val

		default:
			// 尝试使用反射访问结构体字段
			val := reflect.ValueOf(current)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}

			if val.Kind() == reflect.Struct {
				field := val.FieldByName(part)
				if !field.IsValid() {
					return nil, fmt.Errorf("变量 %s 不存在（路径: %s）", varName, strings.Join(parts[:i+1], "."))
				}
				current = field.Interface()
			} else {
				return nil, fmt.Errorf("无法访问变量 %s（路径: %s）", varName, strings.Join(parts[:i+1], "."))
			}
		}
	}

	return current, nil
}

// compareValues 比较两个值
func (e *ConditionExecutor) compareValues(left, right any, operator string) (bool, error) {
	// 处理 null 比较
	if left == nil || right == nil {
		switch operator {
		case "==":
			return left == right, nil
		case "!=":
			return left != right, nil
		default:
			return false, fmt.Errorf("null 值只支持 == 和 != 比较")
		}
	}

	// 类型转换和比较
	switch operator {
	case "==":
		return e.equals(left, right), nil
	case "!=":
		return !e.equals(left, right), nil
	case ">", "<", ">=", "<=":
		return e.compareNumeric(left, right, operator)
	default:
		return false, fmt.Errorf("不支持的操作符: %s", operator)
	}
}

func (e *ConditionExecutor) evaluateFunction(expr string, execCtx *ExecutionContext) (bool, bool, error) {
	switch {
	case strings.HasPrefix(expr, "any_of(") && strings.HasSuffix(expr, ")"):
		args := splitArguments(expr[7 : len(expr)-1])
		for _, arg := range args {
			res, err := e.EvaluateCondition(strings.TrimSpace(arg), execCtx)
			if err != nil {
				return false, true, err
			}
			if res {
				return true, true, nil
			}
		}
		return false, true, nil
	case strings.HasPrefix(expr, "all_of(") && strings.HasSuffix(expr, ")"):
		args := splitArguments(expr[7 : len(expr)-1])
		if len(args) == 0 {
			return false, true, fmt.Errorf("all_of 需要至少一个条件")
		}
		for _, arg := range args {
			res, err := e.EvaluateCondition(strings.TrimSpace(arg), execCtx)
			if err != nil {
				return false, true, err
			}
			if !res {
				return false, true, nil
			}
		}
		return true, true, nil
	case strings.HasPrefix(expr, "none_of(") && strings.HasSuffix(expr, ")"):
		args := splitArguments(expr[8 : len(expr)-1])
		for _, arg := range args {
			res, err := e.EvaluateCondition(strings.TrimSpace(arg), execCtx)
			if err != nil {
				return false, true, err
			}
			if res {
				return false, true, nil
			}
		}
		return true, true, nil
	case strings.HasPrefix(expr, "contains(") && strings.HasSuffix(expr, ")"):
		args := splitArguments(expr[9 : len(expr)-1])
		if len(args) != 2 {
			return false, true, fmt.Errorf("contains 需要两个参数")
		}
		left, err := e.resolveValue(strings.TrimSpace(args[0]), execCtx)
		if err != nil {
			return false, true, err
		}
		right, err := e.resolveValue(strings.TrimSpace(args[1]), execCtx)
		if err != nil {
			return false, true, err
		}
		return containsValue(left, right), true, nil
	case strings.HasPrefix(expr, "empty(") && strings.HasSuffix(expr, ")"):
		val, err := e.resolveValue(strings.TrimSpace(expr[6:len(expr)-1]), execCtx)
		if err != nil {
			return false, true, err
		}
		return lengthOf(val) == 0, true, nil
	case strings.HasPrefix(expr, "not_empty(") && strings.HasSuffix(expr, ")"):
		val, err := e.resolveValue(strings.TrimSpace(expr[10:len(expr)-1]), execCtx)
		if err != nil {
			return false, true, err
		}
		return lengthOf(val) > 0, true, nil
	default:
		return false, false, nil
	}
}

func splitArguments(body string) []string {
	var args []string
	depth := 0
	start := 0
	for i, r := range body {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, body[start:i])
				start = i + 1
			}
		}
	}
	if start < len(body) {
		args = append(args, body[start:])
	}
	return args
}

func containsValue(left, right any) bool {
	switch lv := left.(type) {
	case string:
		return strings.Contains(lv, fmt.Sprintf("%v", right))
	case []any:
		for _, item := range lv {
			if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", right) {
				return true
			}
		}
	case []string:
		for _, item := range lv {
			if item == fmt.Sprintf("%v", right) {
				return true
			}
		}
	case map[string]any:
		if key, ok := right.(string); ok {
			_, exists := lv[key]
			return exists
		}
	}
	return false
}

func lengthOf(value any) int {
	switch v := value.(type) {
	case nil:
		return 0
	case string:
		return len(v)
	case []any:
		return len(v)
	case []string:
		return len(v)
	case map[string]any:
		return len(v)
	case map[string]string:
		return len(v)
	default:
		return 0
	}
}

// equals 判断两个值是否相等
func (e *ConditionExecutor) equals(left, right any) bool {
	// 尝试数字比较
	leftNum, leftIsNum := e.toFloat64(left)
	rightNum, rightIsNum := e.toFloat64(right)
	if leftIsNum && rightIsNum {
		return leftNum == rightNum
	}

	// 字符串比较
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)
	return leftStr == rightStr
}

// compareNumeric 数字比较
func (e *ConditionExecutor) compareNumeric(left, right any, operator string) (bool, error) {
	leftNum, leftOk := e.toFloat64(left)
	rightNum, rightOk := e.toFloat64(right)

	if !leftOk || !rightOk {
		return false, fmt.Errorf("无法将值转换为数字进行比较: %v %s %v", left, operator, right)
	}

	switch operator {
	case ">":
		return leftNum > rightNum, nil
	case "<":
		return leftNum < rightNum, nil
	case ">=":
		return leftNum >= rightNum, nil
	case "<=":
		return leftNum <= rightNum, nil
	default:
		return false, fmt.Errorf("不支持的数字比较操作符: %s", operator)
	}
}

// toFloat64 尝试将值转换为 float64
func (e *ConditionExecutor) toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return num, true
		}
	}
	return 0, false
}

// ExecuteConditional 根据条件执行不同分支
func (e *ConditionExecutor) ExecuteConditional(ctx context.Context, condition *Condition, execCtx *ExecutionContext) (*TaskResult, error) {
	if condition == nil {
		return nil, fmt.Errorf("条件不能为空")
	}

	// 评估条件
	result, err := e.EvaluateCondition(condition.Expression, execCtx)
	if err != nil {
		return nil, fmt.Errorf("条件评估失败: %w", err)
	}

	// 根据条件结果选择执行路径
	var stepID string
	if result {
		stepID = condition.OnTrue
	} else {
		stepID = condition.OnFalse
	}

	if stepID == "" {
		// 如果没有指定步骤，返回成功（跳过）
		return &TaskResult{
			Status: "skipped",
			Output: map[string]any{
				"condition_result": result,
			},
		}, nil
	}

	// 执行对应的步骤
	// 注意：这里需要从工作流定义中查找对应的步骤
	// 简化实现：假设 stepID 可以直接执行
	return &TaskResult{
		Status: "completed",
		Output: map[string]any{
			"condition_result": result,
			"next_step":        stepID,
		},
	}, nil
}

// MatchPattern 模式匹配（正则表达式）
func (e *ConditionExecutor) MatchPattern(pattern, text string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("无效的正则表达式: %w", err)
	}
	return re.MatchString(text), nil
}
