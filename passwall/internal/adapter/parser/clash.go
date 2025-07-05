package parser

import (
	"github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v3"
	"passwall/internal/model"
)

// ClashParser Clash配置解析器
type ClashParser struct{}

// NewClashParser 创建Clash解析器
func NewClashParser() Parser {
	return &ClashParser{}
}

type RawConfig struct {
	Proxies   []map[string]any          `yaml:"proxies"`
	Providers map[string]map[string]any `yaml:"proxy-providers"`
}

// Parse 解析Clash配置
func (p *ClashParser) Parse(content []byte) ([]*model.Proxy, error) {
	// clash有两部分，一个是proxies，用户自己配置
	//一个是proxy-providers，即订阅链接，可以使用share_url.go来解析
	var proxyList []*model.Proxy

	rawCfg := &RawConfig{}
	if err := yaml.Unmarshal(content, rawCfg); err != nil {
		return nil, err
	}
	proxiesConfig := rawCfg.Proxies
	//providersConfig := rawCfg.Providers   // 暂时不解析provider

	for _, proxy := range proxiesConfig {
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

// CanParse 判断是否可以解析Clash配置
func (p *ClashParser) CanParse(content []byte) bool {
	if content == nil {
		return false
	}
	// 简单检查是否包含Clash配置的特征
	rawCfg := &RawConfig{}
	if err := yaml.Unmarshal(content, rawCfg); err != nil {
		return false
	}
	return rawCfg.Proxies != nil
}

func (p *ClashParser) GetType() model.SubscriptionType {
	return model.SubscriptionTypeClash
}
