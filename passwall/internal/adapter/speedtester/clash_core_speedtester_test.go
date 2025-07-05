package speedtester

import (
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewClashCoreSpeedTester 测试ClashCoreSpeedTester的创建
func TestNewClashCoreSpeedTester(t *testing.T) {
	tester := NewClashCoreSpeedTester()
	assert.NotNil(t, tester, "测速器不应为nil")

	// 确保返回的是正确的类型
	_, ok := tester.(*ClashCoreSpeedTester)
	assert.True(t, ok, "应该返回ClashCoreSpeedTester类型")
}

// TestClashCoreSpeedTester_SupportedTypes 测试SupportedTypes方法
func TestClashCoreSpeedTester_SupportedTypes(t *testing.T) {
	tester := NewClashCoreSpeedTester()
	types := tester.SupportedTypes()

	// 确保至少支持VMess类型
	assert.Contains(t, types, model.ProxyTypeVMess, "应该支持VMess代理类型")
}

// TestClashCoreSpeedTester_TestNilProxy 测试Test方法 - 空代理参数
func TestClashCoreSpeedTester_TestNilProxy(t *testing.T) {
	tester := NewClashCoreSpeedTester()
	result, err := tester.Test(nil)

	assert.Nil(t, result, "结果应该为nil")
	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "proxy cannot be nil", "错误消息应该包含'proxy cannot be nil'")
}

// TestClashCoreSpeedTester_TestInvalidConfig 测试Test方法 - 无效的配置JSON
func TestClashCoreSpeedTester_TestInvalidConfig(t *testing.T) {
	tester := NewClashCoreSpeedTester()
	proxy := &model.Proxy{
		Type:   model.ProxyTypeVMess,
		Config: "{invalid json",
	}

	result, err := tester.Test(proxy)

	assert.Nil(t, result, "结果应该为nil")
	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "failed to parse proxy config", "错误消息应该包含'failed to parse proxy config'")
}

// TestClashCoreSpeedTester_TestValidVMess 测试有效的配置 - 使用实际网络连接
func TestClashCoreSpeedTester_TestValidVMess(t *testing.T) {
	t.Skip("此测试需要实际网络连接，默认跳过，需要测速时请取消注释")

	tester := NewClashCoreSpeedTester()
	configString := ""

	proxy := &model.Proxy{
		Type:   model.ProxyTypeVLess,
		Config: configString,
	}

	result, err := tester.Test(proxy)

	assert.NoError(t, err, "测试有效的配置不应该返回错误")
	assert.NotNil(t, result, "结果不应为nil")
	// 打印结果
	t.Logf("Ping: %d ms", result.Ping)
	t.Logf("Download Speed: %d B/s", result.DownloadSpeed)
	t.Logf("Upload Speed: %d B/s", result.UploadSpeed)
}

// TestClashCoreSpeedTester_FactoryIntegration 测试SpeedTester工厂集成
func TestClashCoreSpeedTester_FactoryIntegration(t *testing.T) {
	factory := NewSpeedTesterFactory()
	tester := NewClashCoreSpeedTester()

	// 注册测速器
	factory.RegisterSpeedTester(tester)

	// 获取测速器
	retrievedTester, err := factory.GetSpeedTester(model.ProxyTypeVMess)
	assert.NoError(t, err, "获取已注册的测速器不应该返回错误")
	assert.NotNil(t, retrievedTester, "获取的测速器不应为nil")
}
