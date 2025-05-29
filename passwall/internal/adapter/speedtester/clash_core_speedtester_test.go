package speedtester

import (
	"encoding/json"
	"passwall/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockResult 是一个模拟的测速结果
type MockResult struct {
	mock.Mock
}

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

// TestClashCoreSpeedTester_TestUnsupportedType 测试Test方法 - 不支持的代理类型
func TestClashCoreSpeedTester_TestUnsupportedType(t *testing.T) {
	tester := NewClashCoreSpeedTester()
	proxy := &model.Proxy{
		Type: model.ProxyTypeSS, // 假设SS类型不被支持
	}

	result, err := tester.Test(proxy)

	assert.Nil(t, result, "结果应该为nil")
	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "unsupported proxy type", "错误消息应该包含'unsupported proxy type'")
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

// TestCheckTesterSupport 测试checkTesterSupport函数
func TestCheckTesterSupport(t *testing.T) {
	tester := &ClashCoreSpeedTester{}

	// 测试支持的类型
	supportedProxy := &model.Proxy{
		Type: model.ProxyTypeVMess,
	}
	assert.True(t, checkTesterSupport(supportedProxy, tester), "VMess类型应该被支持")

	// 测试不支持的类型
	unsupportedProxy := &model.Proxy{
		Type: model.ProxyTypeSS,
	}
	assert.False(t, checkTesterSupport(unsupportedProxy, tester), "SS类型不应该被支持")
}

// TestClashCoreSpeedTester_TestValidVMess 测试有效的VMess配置 - 使用实际网络连接
func TestClashCoreSpeedTester_TestValidVMess(t *testing.T) {
	// 跳过实际的网络测试
	t.Skip("此测试需要实际网络连接，默认跳过")

	tester := NewClashCoreSpeedTester()

	// 创建一个有效的VMess配置
	vmessConfig := map[string]interface{}{
		"type":             "vmess",
		"name":             "test-vmess",
		"server":           "example.com",
		"port":             443,
		"uuid":             "00000000-0000-0000-0000-000000000000",
		"alterId":          0,
		"cipher":           "auto",
		"udp":              true,
		"tls":              true,
		"skip-cert-verify": true,
	}

	configJSON, _ := json.Marshal(vmessConfig)

	proxy := &model.Proxy{
		Type:   model.ProxyTypeVMess,
		Config: string(configJSON),
	}

	result, err := tester.Test(proxy)

	assert.NoError(t, err, "测试有效的VMess配置不应该返回错误")
	assert.NotNil(t, result, "结果不应为nil")

	// 检查结果是否包含预期的字段
	assert.GreaterOrEqual(t, result.Ping, 0, "Ping应该大于等于0")
	assert.GreaterOrEqual(t, result.DownloadSpeed, int64(0), "下载速度应该大于等于0")
	assert.GreaterOrEqual(t, result.UploadSpeed, int64(0), "上传速度应该大于等于0")
}

// TestClashCoreSpeedTester_FactoryIntegration 测试SpeedTester工厂集成
func TestClashCoreSpeedTester_FactoryIntegration(t *testing.T) {
	factory := NewSpeedTesterFactory()
	tester := NewClashCoreSpeedTester()

	// 注册测速器
	err := factory.RegisterSpeedTester(tester)
	assert.NoError(t, err, "注册测速器不应该返回错误")

	// 获取测速器
	retrievedTester, err := factory.GetSpeedTester(model.ProxyTypeVMess)
	assert.NoError(t, err, "获取已注册的测速器不应该返回错误")
	assert.NotNil(t, retrievedTester, "获取的测速器不应为nil")

	// 获取不支持的类型
	_, err = factory.GetSpeedTester(model.ProxyTypeSS)
	assert.Error(t, err, "获取未注册的测速器应该返回错误")
}

// TestClashCoreSpeedTester_DuplicateRegistration 测试重复注册测速器
func TestClashCoreSpeedTester_DuplicateRegistration(t *testing.T) {
	factory := NewSpeedTesterFactory()
	tester1 := NewClashCoreSpeedTester()
	tester2 := NewClashCoreSpeedTester()

	// 注册第一个测速器
	err := factory.RegisterSpeedTester(tester1)
	assert.NoError(t, err, "注册第一个测速器不应该返回错误")

	// 尝试注册第二个测速器（应该失败，因为类型已注册）
	err = factory.RegisterSpeedTester(tester2)
	assert.Error(t, err, "重复注册相同类型的测速器应该返回错误")
	assert.Contains(t, err.Error(), "already registered", "错误消息应该包含'already registered'")
}

// TestClashCoreSpeedTester_ProxyLifecycle 测试代理完整生命周期
func TestClashCoreSpeedTester_ProxyLifecycle(t *testing.T) {
	// 创建一个模拟的测试环境
	tester := NewClashCoreSpeedTester()

	// 1. 创建代理
	proxy := &model.Proxy{
		ID:             1,
		Name:           "测试VMess",
		Domain:         "example.com",
		Port:           443,
		Type:           model.ProxyTypeVMess,
		Status:         model.ProxyStatusPending,
		LatestTestTime: nil,
	}

	// 2. 设置有效的配置
	vmessConfig := map[string]interface{}{
		"type":             "vmess",
		"name":             "test-vmess",
		"server":           "example.com",
		"port":             443,
		"uuid":             "00000000-0000-0000-0000-000000000000",
		"alterId":          0,
		"cipher":           "auto",
		"udp":              true,
		"tls":              true,
		"skip-cert-verify": true,
	}
	configJSON, _ := json.Marshal(vmessConfig)
	proxy.Config = string(configJSON)

	// 3. 跳过实际测试，因为它需要网络连接
	t.Skip("此测试需要实际网络连接，默认跳过")

	// 4. 执行测试
	result, err := tester.Test(proxy)

	// 5. 验证结果
	if err != nil {
		t.Logf("测试返回错误: %v", err)
		// 在实际应用中，我们会更新代理状态为失败
		proxy.Status = model.ProxyStatusFailed
	} else {
		// 更新代理信息
		proxy.Ping = result.Ping
		proxy.DownloadSpeed = result.DownloadSpeed
		proxy.UploadSpeed = result.UploadSpeed
		proxy.Status = model.ProxyStatusOK
		now := time.Now()
		proxy.LatestTestTime = &now

		// 验证更新后的代理信息
		assert.Equal(t, model.ProxyStatusOK, proxy.Status, "代理状态应该更新为OK")
		assert.NotNil(t, proxy.LatestTestTime, "最新测试时间不应为nil")
		assert.GreaterOrEqual(t, proxy.Ping, 0, "Ping应该大于等于0")
	}
}

// 测试错误处理和恢复
func TestClashCoreSpeedTester_ErrorHandling(t *testing.T) {
	// 创建测速器
	tester := NewClashCoreSpeedTester()

	// 创建一个配置格式正确但可能导致错误的代理
	// 例如，服务器地址不存在
	vmessConfig := map[string]interface{}{
		"type":             "vmess",
		"name":             "test-vmess-error",
		"server":           "non-existent-server.example", // 不存在的服务器
		"port":             443,
		"uuid":             "00000000-0000-0000-0000-000000000000",
		"alterId":          0,
		"cipher":           "auto",
		"udp":              true,
		"tls":              true,
		"skip-cert-verify": true,
	}
	configJSON, _ := json.Marshal(vmessConfig)

	proxy := &model.Proxy{
		Type:   model.ProxyTypeVMess,
		Config: string(configJSON),
	}

	// 跳过实际测试，因为它需要网络连接
	t.Skip("此测试需要实际网络连接，默认跳过")

	// 执行测试，预期会失败但不应崩溃
	result, err := tester.Test(proxy)

	// 即使测试失败，也应该优雅地处理错误
	if err != nil {
		t.Logf("预期的错误: %v", err)
	} else {
		// 如果测试意外成功，也记录结果
		t.Logf("测试意外成功，结果: Ping=%d, 下载=%d KB/s, 上传=%d KB/s",
			result.Ping, result.DownloadSpeed, result.UploadSpeed)
	}

	// 无论测试成功还是失败，都不应该导致测试本身失败
	// 这只是验证错误处理机制
}

// TestClashCoreSpeedTester_WithMock 使用模拟对象测试
func TestClashCoreSpeedTester_WithMock(t *testing.T) {
	// 这个测试使用模拟对象，不需要实际的网络连接
	t.Skip("此测试需要修改ClashCoreSpeedTester实现以支持依赖注入或使用monkey patching")

	// 以下代码仅用于示例，实际上不会执行
	// 由于我们不能修改原始代码，这里只是展示测试思路

	/*
		// 创建一个有效的VMess配置
		vmessConfig := map[string]interface{}{
			"type":             "vmess",
			"name":             "test-vmess-mock",
			"server":           "example.com",
			"port":             443,
			"uuid":             "00000000-0000-0000-0000-000000000000",
			"alterId":          0,
			"cipher":           "auto",
			"udp":              true,
			"tls":              true,
			"skip-cert-verify": true,
		}
		configJSON, _ := json.Marshal(vmessConfig)

		proxy := &model.Proxy{
			Type:   model.ProxyTypeVMess,
			Config: string(configJSON),
		}

		// 创建预期的测试结果
		expectedResult := &model.SpeedTestResult{
			Ping:          100,
			DownloadSpeed: 1024,
			UploadSpeed:   512,
		}

		// 创建模拟的speedtester
		mockSpeedTester := new(MockSpeedTester)

		// 设置预期行为
		mockSpeedTester.On("TestProxies", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			callback := args.Get(1).(func(*speedtester.Result))
			callback(&speedtester.Result{
				Latency:       time.Duration(expectedResult.Ping) * time.Millisecond,
				DownloadSpeed: float64(expectedResult.DownloadSpeed) / 128,
				UploadSpeed:   float64(expectedResult.UploadSpeed) / 128,
			})
		}).Return()

		// 创建使用模拟对象的测速器
		tester := NewClashCoreSpeedTesterWithDependency(mockSpeedTester)

		// 执行测试
		result, err := tester.Test(proxy)

		// 验证结果
		assert.NoError(t, err, "使用模拟对象的测试不应该返回错误")
		assert.NotNil(t, result, "结果不应为nil")
		assert.Equal(t, expectedResult.Ping, result.Ping, "Ping应该匹配")
		assert.Equal(t, expectedResult.DownloadSpeed, result.DownloadSpeed, "下载速度应该匹配")
		assert.Equal(t, expectedResult.UploadSpeed, result.UploadSpeed, "上传速度应该匹配")

		// 验证模拟对象的方法被调用
		mockSpeedTester.AssertExpectations(t)
	*/
}
