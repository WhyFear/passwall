package util

import (
	"fmt"
	"passwall/internal/model"
)

// GetPasswordFromConfig 根据代理类型从配置中提取用于区分的字段值
// 如果没有找到对应的字段，则返回空字符串
func GetPasswordFromConfig(proxyType model.ProxyType, config map[string]any) string {
	// todo fixme 按实际情况调整
	fieldMap := map[model.ProxyType][]string{
		model.ProxyTypeVMess:     {"uuid"},
		model.ProxyTypeVLess:     {"uuid"},
		model.ProxyTypeTuic:      {"uuid"},
		model.ProxyTypeSS:        {"password"},
		model.ProxyTypeTrojan:    {"password"},
		model.ProxyTypeSocks5:    {"password", "username"},
		model.ProxyTypeHysteria:  {"auth_str"},
		model.ProxyTypeHysteria2: {"password", "auth"},
		model.ProxyTypeSnell:     {"psk"},
		model.ProxyTypeWireGuard: {"reserved"},
		// 其他类型默认为空
	}

	// 获取该代理类型对应的字段列表
	fields, exists := fieldMap[proxyType]
	if !exists {
		// 如果没有定义特定字段，则返回空字符串
		return ""
	}

	// 遍历字段列表，找到第一个存在的非空字段值
	for _, field := range fields {
		// 从配置中获取嵌套的值（支持点号分隔的路径）
		value, exists := GetNestedValue(config, field)
		if exists && IsNotEmpty(value) {
			// 尝试将值转换为字符串
			if strValue, ok := value.(string); ok {
				return strValue
			}
			// 如果不是字符串，转换为字符串返回
			return fmt.Sprintf("%v", value)
		}
	}

	// 如果所有字段都不存在或为空，则返回空字符串
	return ""
}
