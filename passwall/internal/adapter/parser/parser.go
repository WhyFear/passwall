package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"passwall/internal/model"
)

// Parser 解析器接口
type Parser interface {
	// Parse 解析配置内容，返回代理列表
	Parse(content []byte) ([]*model.Proxy, error)
	// CanParse 判断是否可以解析指定内容
	CanParse(content []byte) bool
}

// ParserFactory 解析器工厂
type ParserFactory interface {
	// RegisterParser 注册解析器
	RegisterParser(typeName string, parser Parser)
	// GetParser 获取解析器
	GetParser(typeName string) (Parser, error)
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
func (f *DefaultParserFactory) GetParser(typeName string) (Parser, error) {
	parser, exists := f.parsers[typeName]
	if !exists {
		return nil, fmt.Errorf("parser not found for type: %s", typeName)
	}
	return parser, nil
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
	// 先判断port是否是int类型, float64类型需要转换成int
	if portInt, ok := proxy["port"].(int); ok {
		singleProxy.Port = portInt
	} else if portFloat, ok := proxy["port"].(float64); ok {
		singleProxy.Port = int(portFloat)
	} else {
		port, err := strconv.Atoi(proxy["port"].(string))
		if err != nil {
			err = fmt.Errorf("convert port error: %v", err)
			return nil, err
		}
		singleProxy.Port = port
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
