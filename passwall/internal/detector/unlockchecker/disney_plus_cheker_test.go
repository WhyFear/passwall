package unlockchecker

import (
	"passwall/internal/model"
	"testing"

	"passwall/internal/detector"

	"github.com/stretchr/testify/assert"
)

func TestDisneyPlusChecker_Check(t *testing.T) {
	checker := NewDisneyPlusChecker()

	// 创建一个空的IPProxy用于测试 - 主要测试接口调用
	ipProxy := &detector.IPProxy{
		ProxyClient: nil, // 设置为nil，测试会快速返回fail状态
	}

	// 测试Check方法
	result, err := checker.Check(ipProxy)

	// 由于ProxyClient为nil，我们期望快速返回fail状态
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected a valid CheckResult, got nil")
	}

	// 验证基本的返回结构
	if result.APPName != DisneyPlus {
		t.Errorf("Expected APPName %s, got %s", DisneyPlus, result.APPName)
	}

	t.Logf("DisneyPlus check result: Status=%s, Region=%s", result.Status, result.Region)
}

func TestNewDisneyPlusChecker(t *testing.T) {
	checker := NewDisneyPlusChecker()

	if checker == nil {
		t.Fatal("Expected a valid DisneyPlusChecker, got nil")
	}
	ipProxy := detector.NewIPProxy("1.1.1.1", &model.Proxy{
		Config: "{\"alterId\":1,\"cipher\":\"auto\",\"name\":\"🇭🇰 香港A10 | IEPL\",\"network\":\"ws\",\"port\":13486,\"server\":\"up0m7-g05.hk10-vm5.entry.v50708.dev\",\"skip-cert-verify\":false,\"tls\":false,\"type\":\"vmess\",\"udp\":true,\"uuid\":\"8bb86245-035c-3d9c-b139-b695a8b228d2\",\"ws-opts\":{\"headers\":{\"Host\":\"bgp-01-10.entry-0.chinasnow.net\"},\"path\":\"/\"}}",
	})

	resp, err := checker.Check(ipProxy)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
