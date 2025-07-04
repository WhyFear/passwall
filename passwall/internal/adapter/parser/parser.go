package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/metacubex/mihomo/log"
	"passwall/internal/util"
	"strconv"

	"passwall/internal/model"
)

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
		if h2opts["path"] != nil && len(h2opts["path"].([]string)) == 1 && h2opts["path"].([]string)[0] != "" {
			h2opts["path"] = h2opts["path"].([]string)[0]
		}
	}

	// 整个proxy是一个map，需要转换成json格式
	jsonData, err := json.Marshal(proxy)
	if err != nil {
		err = fmt.Errorf("marshal proxy error: %v", err)
		return nil, err
	}

	singleProxy.Config = string(jsonData)
	return &singleProxy, nil
}
