package risk

import (
	"fmt"
)

// DefaultRiskFactory 默认风险检测器工厂实现
type DefaultRiskFactory struct {
	detectors map[string]Risk
}

// NewRiskFactory 创建风险检测器工厂
func NewRiskFactory() RiskFactory {
	return &DefaultRiskFactory{
		detectors: make(map[string]Risk),
	}
}

// RegisterRiskDetector 注册风险检测器
func (f *DefaultRiskFactory) RegisterRiskDetector(typeName string, risk Risk) {
	f.detectors[typeName] = risk
}

// GetRiskDetector 获取风险检测器
func (f *DefaultRiskFactory) GetRiskDetector(typeName string) (Risk, error) {
	detector, exists := f.detectors[typeName]
	if !exists {
		return nil, fmt.Errorf("risk detector not found for type: %s", typeName)
	}
	return detector, nil
}

// GetAllRiskDetectors 获取所有风险检测器
func (f *DefaultRiskFactory) GetAllRiskDetectors() []Risk {
	allDetectors := make([]Risk, 0, len(f.detectors))
	for _, detector := range f.detectors {
		allDetectors = append(allDetectors, detector)
	}
	return allDetectors
}
