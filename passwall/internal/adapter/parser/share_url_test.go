package parser

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestShareURLParser_Parse(t *testing.T) {
	contentList := []string{
		"hysteria2://e6d8d743-a9bc-48c9-be99-8f3cc94d19e1@asg.gate.top:1008?sni=asg.pagate.top#%F0%9F%87%B8%F0%9F%87%AC%E6%96%B0%E5%8A%A0%E5%9D%A12%20%7C%20%E2%AC%87%EF%B8%8F%205.7MB/s",
		"vmess://ewogICJhZGQiOiAiMTc0OTM4Mjk0MS50ZW5jZW50YXBwLmNuIiwKICAiYWlkIjogMCwKICAiaWQiOiAiNzhkMjdkOTQtNmU1Mi00MzY0LWI0ZDUtNmNjYjA0ZDZlNjliIiwKICAicG9ydCI6IDIwNTMsCiAgInBzIjogIuW+t+Wbvee6veS8puWgoTEgfCAweCIsCiAgInRscyI6IHRydWUsCiAgInR5cGUiOiAibm9uZSIsCiAgInYiOiAiMiIKfQ==",
		"vless://904cdf83-1b0c-4cd5-a0c1-6690c1ee6319@zula.ir:2087?alpn=h2%2Chttp%2F1.1&fp=chrome&host=rita.adaspoloandco.com&path=%2Fdownloader&security=tls&sni=rita.adaspoloandco.com&type=ws#%E6%B1%9F%E6%B1%9F%E7%BE%8E%E5%9B%BD%20IEPL%EF%BD%9C%E5%85%AC%E7%9B%8A%20746",
		"trojan://bd77bfe8-e0f3-11ec-bd7c-f23c913c8d2b@e312e558-sxusg0-t3f7qr-141tv.cu.plebai.net:15229?sni=e312e558-sxusg0-t3f7qr-141tv.cu.plebai.net#%E7%BE%8E%E5%9B%BD%20-%20%E8%8A%9D%E5%8A%A0%E5%93%A5%20-%20Sharktech%20-%2026",
		"ss://YWVzLTI1Ni1jZmI6cXdlclJFV1FAQA==@125.141.31.72:15098#%F0%9F%87%B0%F0%9F%87%B72%20%7C%20%201.5MB/s%7C44%25%7COpenai",
		//"anytls://9f125839-3a31-4d55-89fd-b59d251efa53@sg1.bppp.shop:22311?insecure=1&sni=sg1.bppp.shop#%F0%9F%87%B8%F0%9F%87%AC%E6%96%B0%E5%8A%A0%E5%9D%A15%20%7C%20%E2%AC%87%EF%B8%8F%201.7MB/s",
	}
	content := []byte(strings.Join(contentList, "\n"))

	p := NewShareURLParser()

	proxies, _ := p.Parse(content)

	assert.Equal(t, len(contentList), len(proxies))
}

func TestShareURLParser_CanNotParse(t *testing.T) {
	contentList := []string{
		"anytls://9f125839-3a31-4d55-89fd-b59d251efa53@sg1.bppp.shop:22311?insecure=1&sni=sg1.bppp.shop#%F0%9F%87%B8%F0%9F%87%AC%E6%96%B0%E5%8A%A0%E5%9D%A15%20%7C%20%E2%AC%87%EF%B8%8F%201.7MB/s",
	}
	content := []byte(strings.Join(contentList, "\n"))

	p := NewShareURLParser()

	proxies, _ := p.Parse(content)

	assert.Equal(t, 0, len(proxies))
}

func TestShareURLParser_H2opts(t *testing.T) {
	contentList := []string{
		"vless://a2eda8e1-4452-481b-9a71-aab18d3ed17e@188.245.181.233:8880?security=none&encryption=none&host=eN.TwiTtER.cOM.:8880/?JOKERRVPNBIA_CHANEL@JOKERRVPN&headerType=http&type=tcp#🔥Join+Telegram:@Farah_VPN🟣",
	}
	content := []byte(strings.Join(contentList, "\n"))

	p := NewShareURLParser()

	proxies, _ := p.Parse(content)

	assert.Equal(t, len(contentList), len(proxies))
	for _, proxy := range proxies {
		config := make(map[string]interface{})
		_ = json.Unmarshal([]byte(proxy.Config), &config)

		h2opt := config["h2-opts"].(map[string]interface{})
		assert.Equal(t, "/", h2opt["path"])
	}
}

func TestShareURLParser_CanParseChecker(t *testing.T) {
	content1 := ""
	content2 := "proxies-group:\n  - {name: name, server: 127.0.0.1, port: 443, type: ss, cipher: aes-256-cfb}"
	content3 := "vless://a2eda8e1-4452-481b-9a71-aab18d3ed17e@188.245.181.233:8880?"
	content4 := "c29tZXN0cmluZ3NoZXJl"
	p := NewShareURLParser()
	assert.False(t, p.CanParse([]byte(content1)))
	assert.False(t, p.CanParse([]byte(content2)))
	assert.True(t, p.CanParse([]byte(content3)))
	assert.True(t, p.CanParse([]byte(content4)))
}
