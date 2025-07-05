package generator

import (
	"fmt"
	"strings"

	"passwall/internal/model"
)

// ClashGenerator Clash配置生成器
type ClashGenerator struct{}

// NewClashGenerator 创建Clash配置生成器
func NewClashGenerator() Generator {
	return &ClashGenerator{}
}

// ClashConfig Clash配置结构
type ClashConfig struct {
	Port        int               `yaml:"port"`
	SocksPort   int               `yaml:"socks-port"`
	AllowLan    bool              `yaml:"allow-lan"`
	Mode        string            `yaml:"mode"`
	LogLevel    string            `yaml:"log-level"`
	ExternalUI  string            `yaml:"external-ui,omitempty"`
	Secret      string            `yaml:"secret,omitempty"`
	Proxies     []ClashProxy      `yaml:"proxies"`
	ProxyGroups []ClashProxyGroup `yaml:"proxy-groups,omitempty"`
	Rules       []string          `yaml:"rules,omitempty"`
}

// ClashProxy Clash代理配置
type ClashProxy struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password,omitempty"`
	UUID     string `yaml:"uuid,omitempty"`
	Cipher   string `yaml:"cipher,omitempty"`
	UDP      bool   `yaml:"udp,omitempty"`
	// 其他字段根据需要添加
}

// ClashProxyGroup Clash代理组配置
type ClashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

// Generate 生成Clash配置
func (g *ClashGenerator) Generate(proxies []*model.Proxy) ([]byte, error) {
	// proxies:
	//   - {name: '香港 特别节点', type: anytls, server: 150.241.240.235, port: 10000 ...}
	proxiesYaml := make([]string, 0, len(proxies))
	proxiesYaml = append(proxiesYaml, "proxies:")
	for _, proxy := range proxies {
		proxiesYaml = append(proxiesYaml, fmt.Sprintf(`  - %s`, proxy.Config))
	}

	return []byte(strings.Join(proxiesYaml, "\n")), nil
}

// Format 返回生成的配置格式
func (g *ClashGenerator) Format() string {
	return "clash"
}
