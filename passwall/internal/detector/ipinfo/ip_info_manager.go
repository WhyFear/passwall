package ipinfo

import (
	"fmt"
	"passwall/internal/detector/model"
)

// RiskManager 管理风险检测器
type RiskManager struct {
	factory IPInfoFactory
}

// NewRiskManager 创建风险管理器
func NewRiskManager(factory IPInfoFactory) *RiskManager {
	return &RiskManager{factory: factory}
}

// DetectByAll 调用所有已注册的风险检测器
func (rm *RiskManager) DetectByAll(ipProxy *model.IPProxy) (*map[DetectorName]*IPInfoResult, error) {
	allDetectors := rm.factory.GetAllIPInfoDetectors()
	results := make(map[DetectorName]*IPInfoResult)

	// 并发执行所有检测器
	resultChan := make(chan struct {
		result *IPInfoResult
		err    error
	}, len(allDetectors))

	for _, d := range allDetectors {
		go func(detector IPInfo) {
			result, err := detector.Detect(ipProxy)
			resultChan <- struct {
				result *IPInfoResult
				err    error
			}{result: result, err: err}
		}(d)
	}

	// 等待所有检测器执行完成并处理结果
	for i := 0; i < len(allDetectors); i++ {
		res := <-resultChan
		if res.err != nil {
			// 处理错误，可以选择记录日志或返回错误
			fmt.Printf("检测器 %s 执行失败: %v\n", res.result.Detector, res.err)
		} else if res.result != nil {
			results[res.result.Detector] = res.result
		}
	}

	return &results, nil
}
