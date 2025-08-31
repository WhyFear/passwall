package ipbaseinfo

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetProxyIP(t *testing.T) {
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 测试获取代理IP
	ipInfo, err := GetProxyIP(client)

	// 验证没有错误
	assert.NoError(t, err)
	assert.NotNil(t, ipInfo)

	// 验证至少有一个IP地址
	if ipInfo.IPV4 != "" {
		t.Logf("获取到IPv4地址: %s", ipInfo.IPV4)
		assert.True(t, checkIPV4(ipInfo.IPV4), "IPv4地址格式应该有效")
	}

	if ipInfo.IPV6 != "" {
		t.Logf("获取到IPv6地址: %s", ipInfo.IPV6)
		assert.True(t, checkIPV6(ipInfo.IPV6), "IPv6地址格式应该有效")
	}

	// 至少应该有一个IP地址
	assert.True(t, ipInfo.IPV4 != "" || ipInfo.IPV6 != "", "至少应该获取到一个IP地址")
}

func TestGetAllProxyIPsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 测试实际调用外部URL
	ipInfo, err := GetProxyIP(client)

	if err != nil {
		t.Logf("获取IP失败: %v", err)
		t.Skip("网络连接失败，跳过测试")
	}

	assert.NotNil(t, ipInfo)
	t.Logf("测试结果 - IPv4: %s, IPv6: %s", ipInfo.IPV4, ipInfo.IPV6)
}
