package parser

import (
	"github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v3"
	"passwall/internal/model"

	"github.com/metacubex/mihomo/common/convert"
)

// ShareURLParser 分享链接解析器
type ShareURLParser struct{}

// NewShareURLParser 创建分享链接解析器
func NewShareURLParser() Parser {
	return &ShareURLParser{}
}

// Parse 解析分享链接
func (p *ShareURLParser) Parse(content []byte) ([]*model.Proxy, error) {
	proxies, err := convert.ConvertsV2Ray(content)
	if err != nil {
		return nil, err
	}
	proxyList := make([]*model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		// 转换成proxy格式
		singleProxy, err := parseProxies(proxy)
		if err != nil {
			log.Warnln("parse proxies error: %v", err)
			continue
		}
		proxyList = append(proxyList, singleProxy)
	}
	return proxyList, nil
}

// CanParse 判断是否可以解析分享链接
func (p *ShareURLParser) CanParse(content []byte) bool {
	// mihomo的判断是如果不能yaml解析，就认为是share_url
	if content == nil {
		return false
	}
	rawCfg := &RawConfig{}
	if err := yaml.Unmarshal(content, rawCfg); err != nil {
		return true
	}
	return false
}

// GetName 获取解析器名称
func (p *ShareURLParser) GetType() model.SubscriptionType {
	return model.SubscriptionTypeShareURL
}
