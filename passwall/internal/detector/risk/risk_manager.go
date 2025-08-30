package risk

import (
	"fmt"
	"passwall/internal/detector"
)

// RiskManager 管理风险检测器
type RiskManager struct {
	factory RiskFactory
}

// NewRiskManager 创建风险管理器
func NewRiskManager(factory RiskFactory) *RiskManager {
	return &RiskManager{factory: factory}
}

// DetectByAll 调用所有已注册的风险检测器
func (rm *RiskManager) DetectByAll(ipProxy *detector.IPProxy) (*map[IPRiskDetector]*RiskResult, error) {
	allDetectors := rm.factory.GetAllRiskDetectors()
	results := make(map[IPRiskDetector]*RiskResult)

	// 并发执行所有检测器
	resultChan := make(chan struct {
		detector IPRiskDetector
		result   *RiskResult
		err      error
	}, len(allDetectors))

	for _, d := range allDetectors {
		go func(detector Risk) {
			result, err := detector.Detect(ipProxy)
			// 由于 Risk 接口没有 GetDetectorType 方法，我们需要通过其他方式确定检测器类型
			// 这里我们暂时使用一个占位符，实际项目中应该有更好的方式
			var detectorType IPRiskDetector = IPRiskDetectorUnknown
			if result != nil {
				detectorType = result.Detector
			}
			resultChan <- struct {
				detector IPRiskDetector
				result   *RiskResult
				err      error
			}{
				detector: detectorType,
				result:   result,
				err:      err,
			}
		}(d)
	}

	// 等待所有检测器执行完成并处理结果
	for i := 0; i < len(allDetectors); i++ {
		res := <-resultChan
		if res.err != nil {
			// 处理错误，可以选择记录日志或返回错误
			fmt.Printf("检测器 %s 执行失败: %v\n", res.detector, res.err)
		} else if res.result != nil {
			results[res.detector] = res.result
		}
	}

	return &results, nil
}
