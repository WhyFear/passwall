package util

import (
	"fmt"
	"strings"

	"passwall/internal/model"
)

// ValidationRule 表示一个验证规则
type ValidationRule struct {
	Key       string          // 需要验证的键
	Op        string          // 是否必填，默认为false
	Condition *ValidationRule // 条件，如果不为nil，则只有在条件满足时才验证
	Value     interface{}     // 期望的值，如果为nil，则只验证键是否存在
}

// GetProxyValidationRules 根据代理类型获取验证规则
func GetProxyValidationRules(proxyType model.ProxyType) []ValidationRule {
	// 定义各种代理类型的验证规则
	rules := map[model.ProxyType][]ValidationRule{
		model.ProxyTypeVLess: {
			{Key: "uuid", Op: "exists"},
			{Key: "client-fingerprint", Op: "exists", Condition: &ValidationRule{Key: "reality-opts", Op: "exists"}},
			{Key: "reality-opts.public-key", Op: "exists", Condition: &ValidationRule{Key: "reality-opts", Op: "exists"}},
		},
		model.ProxyTypeSS: {
			{Key: "cipher", Op: "!in", Value: []string{"ss"}},
			{Key: "password", Op: "exists"},
		},
	}
	// 如果没有找到对应的规则，返回空切片
	if _, exists := rules[proxyType]; !exists {
		return []ValidationRule{}
	}

	return rules[proxyType]
}

func ValidateByType(proxyType model.ProxyType, proxy map[string]any) error {
	rules := GetProxyValidationRules(proxyType)
	return ValidateProxyConfig(proxy, rules)
}

// ValidateProxyConfig 验证代理配置是否包含必要的字段
func ValidateProxyConfig(proxy map[string]any, rules []ValidationRule) error {
	for _, rule := range rules {
		// 如果有条件，先检查条件是否满足
		if rule.Condition != nil {
			// 如果条件不满足，跳过此规则
			if !checkCondition(proxy, rule.Condition) {
				continue
			}
		}

		// 检查键是否存在且不为空
		if !checkCondition(proxy, &rule) {
			return fmt.Errorf("当 %s %s 时，字段不满足条件: %s",
				rule.Key, describeOp(&rule), rule.Key)
		}
	}

	return nil
}

// checkCondition 检查条件是否满足
func checkCondition(data map[string]any, condition *ValidationRule) bool {
	value, exists := GetNestedValue(data, condition.Key)

	switch condition.Op {
	case "exists":
		return exists && IsNotEmpty(value)
	case "!exists":
		return !exists || !IsNotEmpty(value)
	case "=":
		return exists && valueEquals(value, condition.Value)
	case "!=":
		return !exists || !valueEquals(value, condition.Value)
	case "in":
		return exists && valueIn(value, condition.Value)
	case "!in":
		return !exists || !valueIn(value, condition.Value)
	default:
		// 默认为存在性检查
		return exists && IsNotEmpty(value)
	}
}

// describeOp 描述条件
func describeOp(condition *ValidationRule) string {
	switch condition.Op {
	case "exists":
		return "应该存在"
	case "!exists":
		return "应该不存在"
	case "=":
		return fmt.Sprintf("= %v", condition.Value)
	case "!=":
		return fmt.Sprintf("!= %v", condition.Value)
	case "in":
		return fmt.Sprintf("应在 %v 中", condition.Value)
	case "!in":
		return fmt.Sprintf("不应在 %v 中", condition.Value)
	default:
		return "存在"
	}
}

// valueEquals 比较两个值是否相等
func valueEquals(a, b interface{}) bool {
	// 如果类型不同，尝试转换
	switch v1 := a.(type) {
	case string:
		if v2, ok := b.(string); ok {
			return v1 == v2
		}
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	case bool:
		if v2, ok := b.(bool); ok {
			return v1 == v2
		}
		if v2, ok := b.(string); ok {
			return fmt.Sprintf("%v", v1) == v2
		}
	case int:
		switch v2 := b.(type) {
		case int:
			return v1 == v2
		case float64:
			return float64(v1) == v2
		case string:
			return fmt.Sprintf("%d", v1) == v2
		}
	case float64:
		switch v2 := b.(type) {
		case int:
			return v1 == float64(v2)
		case float64:
			return v1 == v2
		case string:
			return fmt.Sprintf("%v", v1) == v2
		}
	}

	// 默认比较字符串表示
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// GetNestedValue 获取嵌套的值，支持点号分隔的路径
func GetNestedValue(data map[string]any, path string) (any, bool) {
	// 处理嵌套路径，如 "reality.tls"
	keys := strings.Split(path, ".")
	current := data

	// 遍历除最后一个键外的所有键
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		// 检查当前层级是否存在该键
		if val, ok := current[key]; ok {
			// 检查值是否为map类型
			if nextMap, ok := val.(map[string]any); ok {
				current = nextMap
			} else {
				// 如果不是map类型但不是最后一个键，则无法继续遍历
				return nil, false
			}
		} else {
			// 键不存在
			return nil, false
		}
	}

	// 获取最后一个键的值
	lastKey := keys[len(keys)-1]
	val, exists := current[lastKey]
	return val, exists
}

// IsNotEmpty 检查值是否为空
func IsNotEmpty(value any) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case string:
		return v != ""
	case bool:
		return true // 布尔值总是非空的
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true // 数字总是非空的
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true // 默认认为非空
	}
}

func valueIn(value any, expectValue any) bool {
	// 只支持value为单个值，expectValue为列表或,分割的字符串
	switch v1 := expectValue.(type) {
	case []any:
		for _, v := range v1 {
			if value == v {
				return true
			}
		}
	case []string:
		for _, v := range v1 {
			if value == v {
				return true
			}
		}
	case []int:
		for _, v := range v1 {
			if value == v {
				return true
			}
		}
	case string:
		values := strings.Split(v1, ",")
		for _, v := range values {
			if value == v {
				return true
			}
		}
	}
	return false
}
