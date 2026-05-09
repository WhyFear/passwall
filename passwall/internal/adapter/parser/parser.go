package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/metacubex/mihomo/log"

	"passwall/internal/model"
	"passwall/internal/util"
)

const defaultHysteriaSpeed = 100 * 1024 * 1024

// Parser 解析器接口
type Parser interface {
	// Parse 解析配置内容，返回代理列表
	Parse(content []byte) ([]*model.Proxy, error)
	// CanParse 判断是否可以解析指定内容
	CanParse(content []byte) bool
	// GetType 获取解析器名称
	GetType() model.SubscriptionType
}

// ParserFactory 解析器工厂
type ParserFactory interface {
	// RegisterParser 注册解析器
	RegisterParser(typeName string, parser Parser)
	// GetParser 获取解析器
	GetParser(typeName string, content []byte) (Parser, error)
}

// DefaultParserFactory 默认解析器工厂实现
type DefaultParserFactory struct {
	parsers map[string]Parser
}

// NewParserFactory 创建解析器工厂
func NewParserFactory() ParserFactory {
	return &DefaultParserFactory{
		parsers: make(map[string]Parser),
	}
}

// RegisterParser 注册解析器
func (f *DefaultParserFactory) RegisterParser(typeName string, parser Parser) {
	f.parsers[typeName] = parser
}

// GetParser 获取解析器
func (f *DefaultParserFactory) GetParser(typeName string, content []byte) (Parser, error) {
	switch typeName {
	case "auto":
		return f.AutoDetectParser(content)
	default:
		parser, exists := f.parsers[typeName]
		if !exists {
			return nil, fmt.Errorf("parser not found for type: %s", typeName)
		}
		return parser, nil
	}
}

// AutoDetectParser 自动检测内容类型并返回合适的解析器
func (f *DefaultParserFactory) AutoDetectParser(content []byte) (Parser, error) {
	for _, parser := range f.parsers {
		if parser.CanParse(content) {
			return parser, nil
		}
	}
	return nil, errors.New("no suitable parser found for the content")
}

func parseProxies(proxy map[string]any) (*model.Proxy, error) {
	singleProxy := model.Proxy{}
	singleProxy.Name = proxy["name"].(string)
	singleProxy.Type = model.StringToProxyType(proxy["type"].(string))
	singleProxy.Domain = proxy["server"].(string)
	// 根据不同类型处理端口值
	switch portValue := proxy["port"].(type) {
	case int:
		singleProxy.Port = portValue
	case float64:
		singleProxy.Port = int(portValue)
	case string:
		port, err := strconv.Atoi(portValue)
		if err != nil {
			return nil, fmt.Errorf("解析端口错误: %v", err)
		}
		singleProxy.Port = port
	case nil:
		return nil, fmt.Errorf("端口值不能为空")
	default:
		return nil, fmt.Errorf("不支持的端口类型: %T", proxy["port"])
	}

	if err := util.ValidateByType(singleProxy.Type, proxy); err != nil {
		log.Errorln("校验代理配置失败: %v，domain=%v, port=%v", err, singleProxy.Domain, singleProxy.Port)
		return nil, fmt.Errorf("校验代理配置失败: %v", err)
	}

	// fixme 特化处理一下:[ proxy 'h2-opts.path' expected type 'string', got unconvertible type '[]interface {}'" ]
	if proxy["h2-opts"] != nil {
		h2opts := proxy["h2-opts"].(map[string]any)
		// 处理path字段，支持[]string和string两种类型
		if h2opts["path"] != nil {
			switch pathValue := h2opts["path"].(type) {
			case string:
				// 已经是字符串类型，无需处理
			case []string:
				if len(pathValue) > 0 && pathValue[0] != "" {
					h2opts["path"] = pathValue[0]
				}
			default:
				log.Errorln("不支持的h2-opts.path类型: %T", h2opts["path"])
			}
		}
	}
	// fixme 特化处理hysteria的up和down在mihomo里不能为0，且不能为科学计数法的问题
	if singleProxy.Type == model.ProxyTypeHysteria || singleProxy.Type == model.ProxyTypeHysteria2 {
		defaultZero := singleProxy.Type == model.ProxyTypeHysteria
		normalizeHysteriaSpeedField(proxy, "up", defaultZero)
		normalizeHysteriaSpeedField(proxy, "down", defaultZero)
	}

	// 提取密码字段用于唯一键区分
	singleProxy.Password = util.GetPasswordFromConfig(singleProxy.Type, proxy)

	// 整个proxy是一个map，需要转换成json格式
	jsonData, err := json.Marshal(proxy)
	if err != nil {
		err = fmt.Errorf("marshal proxy error: %v", err)
		return nil, err
	}

	singleProxy.Config = string(jsonData)
	return &singleProxy, nil
}

func normalizeHysteriaSpeedField(proxy map[string]any, field string, defaultZero bool) {
	value, exists := proxy[field]
	if !exists || value == nil {
		return
	}

	normalized, ok := normalizeHysteriaSpeedValue(value, defaultZero)
	if !ok {
		log.Errorln("不支持的hysteria.%s类型: %T", field, value)
		return
	}
	proxy[field] = normalized
}

func normalizeHysteriaSpeedValue(value any, defaultZero bool) (any, bool) {
	switch v := value.(type) {
	case int:
		return normalizeHysteriaIntegerSpeed(int64(v), defaultZero), true
	case int64:
		return normalizeHysteriaIntegerSpeed(v, defaultZero), true
	case float64:
		return normalizeHysteriaFloatSpeed(v, defaultZero), true
	case float32:
		return normalizeHysteriaFloatSpeed(float64(v), defaultZero), true
	case string:
		return normalizeHysteriaStringSpeed(v, defaultZero), true
	default:
		return nil, false
	}
}

func normalizeHysteriaIntegerSpeed(value int64, defaultZero bool) any {
	if value == 0 && defaultZero {
		return defaultHysteriaSpeed
	}
	return value
}

func normalizeHysteriaFloatSpeed(value float64, defaultZero bool) any {
	if value == 0 && defaultZero {
		return defaultHysteriaSpeed
	}
	if normalized, ok := decimalIntegerSpeed(value); ok {
		return normalized
	}
	return value
}

func normalizeHysteriaStringSpeed(value string, defaultZero bool) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if defaultZero {
			return strconv.Itoa(defaultHysteriaSpeed)
		}
		return value
	}

	if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
		if parsed == 0 && defaultZero {
			return strconv.Itoa(defaultHysteriaSpeed)
		}
		if normalized, ok := decimalIntegerSpeed(parsed); ok {
			return normalized
		}
	}

	return trimmed
}

func decimalIntegerSpeed(value float64) (string, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value != math.Trunc(value) {
		return "", false
	}
	return strconv.FormatFloat(value, 'f', 0, 64), true
}
