package parser

import (
	"github.com/metacubex/mihomo/log"
	"strings"

	"passwall/internal/model"

	"github.com/metacubex/mihomo/common/convert"
)

// 文件可能是base64编码的链接，也可能是未base64编码的链接
// 解码后的数据示例：
//trojan://2847f31d-c9ac-41cf-b4f5-39a4505a7765@hk1.72adf0e1-194c-5db0-87d4-ed99f25f0d61.6df03129.the-best-airport.com:443?allowInsecure=1&peer=new.download.the-best-airport.com&sni=new.download.the-best-airport.com&type=tcp#%F0%9F%87%AD%F0%9F%87%B0%E9%A6%99%E6%B8%AF%2001%20%7C%20%E4%B8%93%E7%BA%BF
//vless://745e818d-38d1-46d6-8dfd-9b0e1d66ad1a@iij.pakro.top:8887?encryption=none&flow=xtls-rprx-vision&security=reality&sni=icloud.cdn-apple.com&fp=chrome&pbk=S-g0oP36DShii1uPOnZDSEhp_wQghX6h68PgMivOmD4&allowInsecure=1&type=tcp&headerType=none#mine日本iij
//根据前缀判断类型并解析出单个服务器

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
	//	fmt.Println(proxies)
	// 解析出单个服务器
	proxyList := make([]*model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		// 转换成proxy格式
		singleProxy, err := parseProxies(proxy)
		if err != nil {
			log.Warnln("parse proxies error: %v", err)
		}
		proxyList = append(proxyList, singleProxy)
	}
	return proxyList, nil
}

// CanParse 判断是否可以解析分享链接
func (p *ShareURLParser) CanParse(content []byte) bool {
	contentStr := string(content)

	// 检查是否包含常见的代理协议前缀
	return strings.Contains(contentStr, "vmess://") ||
		strings.Contains(contentStr, "vless://") ||
		strings.Contains(contentStr, "trojan://") ||
		strings.Contains(contentStr, "ss://") ||
		strings.Contains(contentStr, "ssr://") ||
		strings.Contains(contentStr, "socks://") ||
		strings.Contains(contentStr, "socks5://")
}
