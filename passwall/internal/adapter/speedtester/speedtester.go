package speedtester

import (
	"fmt"
	"passwall/internal/model"
)

// SpeedTester 测速器接口
type SpeedTester interface {
	// Test 测试代理速度
	Test(proxy *model.Proxy) (*model.SpeedTestResult, error)

	// SupportedTypes 返回支持的代理类型列表
	SupportedTypes() []model.ProxyType
}

// SpeedTesterFactory 测速器工厂接口
type SpeedTesterFactory interface {
	// GetSpeedTester 获取指定类型的测速器
	GetSpeedTester(proxyType model.ProxyType) (SpeedTester, error)

	// RegisterSpeedTester 注册测速器
	RegisterSpeedTester(tester SpeedTester)
}

// speedTesterFactoryImpl 测速器工厂实现
type speedTesterFactoryImpl struct {
	typeToTester map[model.ProxyType]SpeedTester
}

// NewSpeedTesterFactory 创建测速器工厂
func NewSpeedTesterFactory() SpeedTesterFactory {
	return &speedTesterFactoryImpl{
		typeToTester: make(map[model.ProxyType]SpeedTester),
	}
}

// GetSpeedTester 获取指定类型的测速器
func (f *speedTesterFactoryImpl) GetSpeedTester(proxyType model.ProxyType) (SpeedTester, error) {
	tester, exists := f.typeToTester[proxyType]
	if !exists {
		return nil, fmt.Errorf("unsupported proxy type for speed testing: %s", proxyType)
	}
	return tester, nil
}

// RegisterSpeedTester 注册测速器
func (f *speedTesterFactoryImpl) RegisterSpeedTester(tester SpeedTester) {
	if tester == nil {
		return
	}

	// 遍历测速器支持的所有类型进行注册
	for _, proxyType := range tester.SupportedTypes() {
		if _, exists := f.typeToTester[proxyType]; exists {
			continue
		}
		f.typeToTester[proxyType] = tester
	}
}
