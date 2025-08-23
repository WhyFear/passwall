package detector

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// detectorManager 检测器管理器实现
type detectorManager struct {
	detectors map[string]Detector
	mu        sync.RWMutex
	eventBus  EventBus
	metrics   MetricsCollector
}

// NewDetectorManager 创建新的检测器管理器
func NewDetectorManager(eventBus EventBus, metrics MetricsCollector) DetectorManager {
	return &detectorManager{
		detectors: make(map[string]Detector),
		eventBus:  eventBus,
		metrics:   metrics,
	}
}

// RegisterDetector 注册检测器
func (dm *detectorManager) RegisterDetector(detector Detector) error {
	if detector == nil {
		return fmt.Errorf("detector cannot be nil")
	}

	detectorType := string(detector.GetType())

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.detectors[detectorType]; exists {
		return fmt.Errorf("detector %s already registered", detectorType)
	}

	dm.detectors[detectorType] = detector

	// 发布注册事件
	if dm.eventBus != nil {
		event := &DetectorEvent{
			Type:      "detector.registered",
			Detector:  detectorType,
			Status:    "success",
			Message:   fmt.Sprintf("Detector %s registered successfully", detectorType),
			Timestamp: time.Now(),
		}
		_ = dm.eventBus.Publish(event)
	}

	return nil
}

// UnregisterDetector 注销检测器
func (dm *detectorManager) UnregisterDetector(detectorType string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.detectors[detectorType]; !exists {
		return fmt.Errorf("detector %s not found", detectorType)
	}

	delete(dm.detectors, detectorType)

	// 发布注销事件
	if dm.eventBus != nil {
		event := &DetectorEvent{
			Type:      "detector.unregistered",
			Detector:  detectorType,
			Status:    "success",
			Message:   fmt.Sprintf("Detector %s unregistered successfully", detectorType),
			Timestamp: time.Now(),
		}
		_ = dm.eventBus.Publish(event)
	}

	return nil
}

// GetDetector 获取检测器
func (dm *detectorManager) GetDetector(detectorType string) (Detector, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	detector, exists := dm.detectors[detectorType]
	if !exists {
		return nil, fmt.Errorf("detector %s not found", detectorType)
	}

	return detector, nil
}

// GetAllDetectors 获取所有检测器
func (dm *detectorManager) GetAllDetectors() []Detector {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	detectors := make([]Detector, 0, len(dm.detectors))
	for _, detector := range dm.detectors {
		detectors = append(detectors, detector)
	}

	return detectors
}

// GetEnabledDetectors 获取已启用的检测器
func (dm *detectorManager) GetEnabledDetectors() []Detector {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var enabledDetectors []Detector
	for _, detector := range dm.detectors {
		if detector.IsEnabled() {
			enabledDetectors = append(enabledDetectors, detector)
		}
	}

	return enabledDetectors
}

// TestAllDetectors 测试所有检测器
func (dm *detectorManager) TestAllDetectors(ctx context.Context) map[string]error {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	results := make(map[string]error)

	for detectorType, detector := range dm.detectors {
		if !detector.IsEnabled() {
			continue
		}

		err := detector.TestConnection(ctx)
		results[detectorType] = err

		// 记录指标
		if dm.metrics != nil {
			if err != nil {
				dm.metrics.RecordError(detectorType, "connection_test")
			}
		}

		// 发布测试事件
		if dm.eventBus != nil {
			status := "success"
			message := fmt.Sprintf("Detector %s connection test successful", detectorType)
			if err != nil {
				status = "error"
				message = fmt.Sprintf("Detector %s connection test failed: %v", detectorType, err)
			}

			event := &DetectorEvent{
				Type:      "detector.tested",
				Detector:  detectorType,
				Status:    status,
				Message:   message,
				Timestamp: time.Now(),
			}
			_ = dm.eventBus.Publish(event)
		}
	}

	return results
}

// DetectAll 使用所有检测器检测IP
func (dm *detectorManager) DetectAll(ctx context.Context, ip string) map[string]*DetectorResult {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	results := make(map[string]*DetectorResult)

	for detectorType, detector := range dm.detectors {
		if !detector.IsEnabled() {
			continue
		}

		startTime := time.Now()
		result, err := detector.Detect(ctx, ip)
		duration := time.Since(startTime)

		// 记录指标
		if dm.metrics != nil {
			dm.metrics.RecordDetection(detectorType, err == nil, duration)
			dm.metrics.RecordLatency(detectorType, duration)
			if err != nil {
				dm.metrics.RecordError(detectorType, "detection")
			}
		}

		// 设置结果
		if result == nil {
			result = &DetectorResult{
				Success:   false,
				Error:     err,
				Timestamp: time.Now(),
				Provider:  detector.GetProvider(),
			}
		}

		results[detectorType] = result

		// 发布检测事件
		if dm.eventBus != nil {
			status := "success"
			message := fmt.Sprintf("Detector %s completed detection for IP %s", detectorType, ip)
			if err != nil {
				status = "error"
				message = fmt.Sprintf("Detector %s failed to detect IP %s: %v", detectorType, ip, err)
			}

			event := &DetectorEvent{
				Type:     "detection.completed",
				Detector: detectorType,
				IP:       ip,
				Status:   status,
				Message:  message,
				Data: map[string]interface{}{
					"duration": duration,
					"result":   result,
				},
				Timestamp: time.Now(),
			}
			_ = dm.eventBus.Publish(event)
		}
	}

	return results
}

// BatchDetect 批量检测
func (dm *detectorManager) BatchDetect(ctx context.Context, ips []string, options *BatchDetectOptions) map[string]*BatchDetectResult {
	if options == nil {
		options = &BatchDetectOptions{
			Concurrency:     5,
			Timeout:         30 * time.Second,
			RetryCount:      3,
			RetryDelay:      1 * time.Second,
			ContinueOnError: true,
		}
	}

	results := make(map[string]*BatchDetectResult)

	// 创建工作池
	jobs := make(chan string, len(ips))
	resultsChan := make(chan *BatchDetectResult, len(ips))

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < options.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				result := dm.batchDetectSingleIP(ctx, ip, options)
				resultsChan <- result
			}
		}()
	}

	// 发送任务
	go func() {
		for _, ip := range ips {
			jobs <- ip
		}
		close(jobs)
	}()

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果
	for result := range resultsChan {
		results[result.IP] = result
	}

	return results
}

// batchDetectSingleIP 批量检测单个IP
func (dm *detectorManager) batchDetectSingleIP(ctx context.Context, ip string, options *BatchDetectOptions) *BatchDetectResult {
	startTime := time.Now()

	// 获取要使用的检测器
	detectors := dm.GetEnabledDetectors()
	if len(options.DetectorTypes) > 0 {
		var filteredDetectors []Detector
		for _, detector := range detectors {
			for _, allowedType := range options.DetectorTypes {
				if detector.GetType() == allowedType {
					filteredDetectors = append(filteredDetectors, detector)
					break
				}
			}
		}
		detectors = filteredDetectors
	}

	// 执行检测
	results := make(map[string]*DetectorResult)
	var totalDuration time.Duration
	var successCount, failureCount int
	var minLatency, maxLatency time.Duration

	for _, detector := range detectors {
		detectorStart := time.Now()

		// 重试逻辑
		var result *DetectorResult
		var err error
		for i := 0; i < options.RetryCount; i++ {
			if i > 0 {
				select {
				case <-time.After(options.RetryDelay):
				case <-ctx.Done():
					break
				}
			}

			result, err = detector.Detect(ctx, ip)
			if err == nil || !options.ContinueOnError {
				break
			}
		}

		duration := time.Since(detectorStart)
		totalDuration += duration

		if result == nil {
			result = &DetectorResult{
				Success:   false,
				Error:     err,
				Timestamp: time.Now(),
				Provider:  detector.GetProvider(),
			}
		}

		results[string(detector.GetType())] = result

		if result.Success {
			successCount++
		} else {
			failureCount++
		}

		// 更新延迟统计
		if minLatency == 0 || duration < minLatency {
			minLatency = duration
		}
		if duration > maxLatency {
			maxLatency = duration
		}

		// 记录指标
		if dm.metrics != nil {
			dm.metrics.RecordDetection(string(detector.GetType()), result.Success, duration)
			dm.metrics.RecordLatency(string(detector.GetType()), duration)
			if !result.Success {
				dm.metrics.RecordError(string(detector.GetType()), "batch_detection")
			}
		}
	}

	// 计算平均延迟
	var avgLatency time.Duration
	if len(results) > 0 {
		avgLatency = totalDuration / time.Duration(len(results))
	}

	// 创建摘要
	summary := &BatchDetectSummary{
		TotalDetectors:  len(results),
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		AverageResponse: avgLatency,
		MinLatency:      minLatency,
		MaxLatency:      maxLatency,
	}

	return &BatchDetectResult{
		IP:        ip,
		Results:   results,
		Summary:   summary,
		Duration:  time.Since(startTime),
		Timestamp: time.Now(),
	}
}
