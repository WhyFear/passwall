package parser

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestClashParser_Parse(t *testing.T) {
	contentList := []string{
		"{name: name, server: 127.0.0.1, port: 444, type: trojan, password: password, sni: www.baidu.com, skip-cert-verify: true}",
		"alpn:\n      - h3\n    auth_str: baidu.com\n    down: ''\n    name: test\n    obfs: ''\n    port: 46938\n    protocol: ''\n    server: 127.0.0.1\n    skip-cert-verify: true\n    sni: apple.com\n    type: hysteria\n    up: ''\n    auth-str: baidu.com\n    delay: 2031",
	}
	proxiesYaml := make([]string, 0, len(contentList))
	proxiesYaml = append(proxiesYaml, "proxies:")
	for _, proxy := range contentList {
		proxiesYaml = append(proxiesYaml, fmt.Sprintf(`  - %s`, proxy))
	}
	content := []byte(strings.Join(proxiesYaml, "\n"))

	p := NewClashParser()

	proxies, _ := p.Parse(content)

	assert.Equal(t, len(contentList), len(proxies))
}

func TestClashParser_CanNotParse(t *testing.T) {
	contentList := []string{
		"{name: name, server: 127.0.0.1, port: 443, type: ss, cipher: ss, password: password}",
		"{name: name, server: 127.0.0.1, port: 443, type: ss, cipher: aes-256-cfb}",
	}
	proxiesYaml := make([]string, 0, len(contentList))
	proxiesYaml = append(proxiesYaml, "proxies:")
	for _, proxy := range contentList {
		proxiesYaml = append(proxiesYaml, fmt.Sprintf(`  - %s`, proxy))
	}
	content := []byte(strings.Join(proxiesYaml, "\n"))

	p := NewClashParser()

	proxies, _ := p.Parse(content)

	assert.Equal(t, 0, len(proxies))
}

func TestClashParser_CanParse(t *testing.T) {
	content := "proxies:\n  - {name: name, server: 127.0.0.1, port: 443, type: ss, cipher: aes-256-cfb}"
	p := NewClashParser()
	assert.True(t, p.CanParse([]byte(content)))
}

func TestClashParser_CantParse(t *testing.T) {
	content1 := ""
	content2 := "vless://a2eda8e1-4452-481b-9a71-aab18d3ed17e@188.245.181.233:8880?"
	content3 := "proxies-group:\n  - {name: name, server: 127.0.0.1, port: 443, type: ss, cipher: aes-256-cfb}"
	p := NewClashParser()
	assert.False(t, p.CanParse([]byte(content1)))
	assert.False(t, p.CanParse([]byte(content2)))
	assert.False(t, p.CanParse([]byte(content3)))
}
