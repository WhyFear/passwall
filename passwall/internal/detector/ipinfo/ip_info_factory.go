package ipinfo

import (
	"fmt"
)

// DefaultRiskFactory 默认风险检测器工厂实现
type DefaultRiskFactory struct {
	detectors map[string]IPInfo
}

// NewRiskFactory 创建风险检测器工厂
func NewRiskFactory() IPInfoFactory {
	return &DefaultRiskFactory{
		detectors: make(map[string]IPInfo),
	}
}

// RegisterRiskDetector 注册风险检测器
func (f *DefaultRiskFactory) RegisterIPInfoDetector(typeName string, risk IPInfo) {
	f.detectors[typeName] = risk
}

// GetRiskDetector 获取风险检测器
func (f *DefaultRiskFactory) GetIPInfoDetector(typeName string) (IPInfo, error) {
	detector, exists := f.detectors[typeName]
	if !exists {
		return nil, fmt.Errorf("ipinfo detector not found for type: %s", typeName)
	}
	return detector, nil
}

// GetAllRiskDetectors 获取所有风险检测器
func (f *DefaultRiskFactory) GetAllIPInfoDetectors() []IPInfo {
	allDetectors := make([]IPInfo, 0, len(f.detectors))
	for _, detector := range f.detectors {
		allDetectors = append(allDetectors, detector)
	}
	return allDetectors
}
