package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"passwall/internal/detector"
	"passwall/internal/model"
)

// IPQualityServiceConfig IP质量服务配置
type IPQualityServiceConfig struct {
	Enabled                bool          `yaml:"enabled" json:"enabled"`
	Timeout                time.Duration `yaml:"timeout" json:"timeout"`
	MaxConcurrentDetectors int           `yaml:"max_concurrent_detectors" json:"max_concurrent_detectors"`
	EnableGeolocation      bool          `yaml:"enable_geolocation" json:"enable_geolocation"`
	EnableRiskAssessment   bool          `yaml:"enable_risk_assessment" json:"enable_risk_assessment"`
	EnableServiceDetection bool          `yaml:"enable_service_detection" json:"enable_service_detection"`
}

// IPQualityResult IP质量检测综合结果
type IPQualityResult struct {
	IP                string                              `json:"ip"`
	OverallStatus     model.IPQualityStatus               `json:"overall_status"`
	OverallScore      float64                             `json:"overall_score"`
	GeolocationResult *detector.GeolocationResult         `json:"geolocation_result,omitempty"`
	RiskResults       []*detector.RiskAssessmentResult    `json:"risk_results,omitempty"`
	ServiceResults    []*detector.ServiceUnlockResult     `json:"service_results,omitempty"`
	Summary           *IPQualitySummary                   `json:"summary"`
	Timestamp         time.Time                           `json:"timestamp"`
	Duration          time.Duration                       `json:"duration"`
	DetectorResults   map[string]*detector.DetectorResult `json:"detector_results,omitempty"`
}

// IPQualitySummary IP质量摘要
type IPQualitySummary struct {
	TotalDetectors      int      `json:"total_detectors"`
	SuccessfulDetectors int      `json:"successful_detectors"`
	FailedDetectors     int      `json:"failed_detectors"`
	IsVPN               bool     `json:"is_vpn"`
	IsProxy             bool     `json:"is_proxy"`
	IsTor               bool     `json:"is_tor"`
	IsHighRisk          bool     `json:"is_high_risk"`
	AvailableServices   int      `json:"available_services"`
	BlockedServices     int      `json:"blocked_services"`
	Country             string   `json:"country"`
	City                string   `json:"city"`
	ISP                 string   `json:"isp"`
	RiskLevel           string   `json:"risk_level"`
	Recommendations     []string `json:"recommendations,omitempty"`
}

// IPQualityService IP质量检测服务
type IPQualityService struct {
	config          *IPQualityServiceConfig
	detectorManager detector.DetectorManager
	cache           map[string]*CacheEntry
	cacheMutex      sync.RWMutex
	mu              sync.RWMutex
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Result    *IPQualityResult `json:"result"`
	ExpiresAt time.Time        `json:"expires_at"`
}

// NewIPQualityService 创建IP质量检测服务
func NewIPQualityService(config *IPQualityServiceConfig, manager detector.DetectorManager) (*IPQualityService, error) {
	if config == nil {
		config = &IPQualityServiceConfig{
			Enabled:                true,
			Timeout:                60 * time.Second,
			MaxConcurrentDetectors: 10,
			EnableGeolocation:      true,
			EnableRiskAssessment:   true,
			EnableServiceDetection: true,
		}
	}

	service := &IPQualityService{
		config:          config,
		detectorManager: manager,
	}

	return service, nil
}

// DetectIPQuality 检测IP质量
func (s *IPQualityService) DetectIPQuality(ctx context.Context, ip string, proxyID uint) (*IPQualityResult, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("IP quality service is disabled")
	}

	startTime := time.Now()

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	result := &IPQualityResult{
		IP:              ip,
		Timestamp:       startTime,
		DetectorResults: make(map[string]*detector.DetectorResult),
		Summary: &IPQualitySummary{
			Recommendations: make([]string, 0),
		},
	}

	// 并发执行所有启用的检测器
	var wg sync.WaitGroup
	var mu sync.Mutex
	var detectorErrors []error

	// 地理位置检测
	if s.config.EnableGeolocation {
		if detectors := s.getDetectorsByType(detector.DetectorTypeGeolocation); len(detectors) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				geoResult, err := s.detectGeolocation(ctx, proxyID, detectors)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					detectorErrors = append(detectorErrors, fmt.Errorf("geolocation detection failed: %w", err))
				} else if geoResult != nil {
					result.GeolocationResult = geoResult
				}
			}()
		}
	}

	// 风险评估检测
	if s.config.EnableRiskAssessment {
		if detectors := s.getDetectorsByType(detector.DetectorTypeRisk); len(detectors) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				riskResults, err := s.detectRisk(ctx, ip, detectors)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					detectorErrors = append(detectorErrors, fmt.Errorf("risk assessment failed: %w", err))
				} else if len(riskResults) > 0 {
					result.RiskResults = riskResults
				}
			}()
		}
	}

	// 服务解锁检测
	if s.config.EnableServiceDetection {
		if detectors := s.getDetectorsByType(detector.DetectorTypeService); len(detectors) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				serviceResults, err := s.detectServices(ctx, ip, detectors)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					detectorErrors = append(detectorErrors, fmt.Errorf("service detection failed: %w", err))
				} else if serviceResults != nil {
					result.ServiceResults = serviceResults
				}
			}()
		}
	}

	// 等待所有检测完成
	wg.Wait()

	// 计算检测时长
	result.Duration = time.Since(startTime)

	// 生成综合结果
	s.generateSummary(result)
	s.calculateOverallScore(result)
	s.determineOverallStatus(result)

	// 如果所有检测器都失败了，返回错误
	if len(detectorErrors) > 0 && result.GeolocationResult == nil && len(result.RiskResults) == 0 && len(result.ServiceResults) == 0 {
		return nil, fmt.Errorf("all detectors failed: %v", detectorErrors)
	}

	return result, nil
}

// detectGeolocation 执行地理位置检测
func (s *IPQualityService) detectGeolocation(ctx context.Context, ip string, detectors []detector.Detector, proxyID uint) (*detector.GeolocationResult, error) {
	// 优先使用第一个可用的地理位置检测器
	for _, det := range detectors {
		if geoDet, ok := det.(detector.GeolocationDetector); ok && det.IsEnabled() {
			result, err := geoDet.DetectGeolocation(ctx, proxyID)
			if err == nil {
				return result, nil
			}
		}
	}
	return nil, fmt.Errorf("no geolocation detector available")
}

// detectRisk 执行风险评估检测
func (s *IPQualityService) detectRisk(ctx context.Context, ip string, detectors []detector.Detector) ([]*detector.RiskAssessmentResult, error) {
	var allResults []*detector.RiskAssessmentResult

	// 收集所有风险检测器的结果
	for _, det := range detectors {
		if riskDet, ok := det.(detector.RiskDetector); ok && det.IsEnabled() {
			result, err := riskDet.DetectRisk(ctx, proxyID)
			if err == nil {
				allResults = append(allResults, result)
			}
		}
	}

	if len(allResults) == 0 {
		return nil, fmt.Errorf("no risk detector available")
	}

	return allResults, nil
}

// detectServices 执行服务解锁检测
func (s *IPQualityService) detectServices(ctx context.Context, ip string, detectors []detector.Detector) ([]*detector.ServiceUnlockResult, error) {
	var allResults []*detector.ServiceUnlockResult

	// 合并所有服务检测器的结果
	for _, det := range detectors {
		if serviceDet, ok := det.(detector.ServiceDetector); ok && det.IsEnabled() {
			results, err := serviceDet.DetectService(ctx, proxyID)
			if err == nil {
				allResults = append(allResults, results...)
			}
		}
	}

	if len(allResults) == 0 {
		return nil, fmt.Errorf("no service detector available")
	}

	return allResults, nil
}

// mergeGeolocationResults 合并多个地理位置检测结果
func (s *IPQualityService) mergeGeolocationResults(results []*detector.GeolocationResult) *detector.GeolocationResult {
	if len(results) == 0 {
		return nil
	}

	merged := &detector.GeolocationResult{}

	// 统计各字段出现频率，选择最常见的值
	countryCount := make(map[string]int)
	countryCodeCount := make(map[string]int)
	regionCount := make(map[string]int)
	regionCodeCount := make(map[string]int)
	cityCount := make(map[string]int)
	zipCodeCount := make(map[string]int)
	timezoneCount := make(map[string]int)
	ispCount := make(map[string]int)
	asnCount := make(map[string]int)
	organizationCount := make(map[string]int)

	var totalLat, totalLon float64
	var validCoords int

	for _, result := range results {
		if result.Country != "" {
			countryCount[result.Country]++
		}
		if result.CountryCode != "" {
			countryCodeCount[result.CountryCode]++
		}
		if result.Region != "" {
			regionCount[result.Region]++
		}
		if result.RegionCode != "" {
			regionCodeCount[result.RegionCode]++
		}
		if result.City != "" {
			cityCount[result.City]++
		}
		if result.ZipCode != "" {
			zipCodeCount[result.ZipCode]++
		}
		if result.Timezone != "" {
			timezoneCount[result.Timezone]++
		}
		if result.ISP != "" {
			ispCount[result.ISP]++
		}
		if result.ASN != "" {
			asnCount[result.ASN]++
		}
		if result.Organization != "" {
			organizationCount[result.Organization]++
		}

		// 坐标取平均值
		if result.Latitude != 0 && result.Longitude != 0 {
			totalLat += result.Latitude
			totalLon += result.Longitude
			validCoords++
		}

		// 布尔值使用OR逻辑
		merged.IsVPN = merged.IsVPN || result.IsVPN
		merged.IsProxy = merged.IsProxy || result.IsProxy
		merged.IsHosting = merged.IsHosting || result.IsHosting
		merged.IsTor = merged.IsTor || result.IsTor
	}

	// 选择最常见的值
	merged.Country = getMostFrequent(countryCount)
	merged.CountryCode = getMostFrequent(countryCodeCount)
	merged.Region = getMostFrequent(regionCount)
	merged.RegionCode = getMostFrequent(regionCodeCount)
	merged.City = getMostFrequent(cityCount)
	merged.ZipCode = getMostFrequent(zipCodeCount)
	merged.Timezone = getMostFrequent(timezoneCount)
	merged.ISP = getMostFrequent(ispCount)
	merged.ASN = getMostFrequent(asnCount)
	merged.Organization = getMostFrequent(organizationCount)

	// 计算平均坐标
	if validCoords > 0 {
		merged.Latitude = totalLat / float64(validCoords)
		merged.Longitude = totalLon / float64(validCoords)
	}

	return merged
}

// getMostFrequent 获取出现频率最高的值
func getMostFrequent(counts map[string]int) string {
	var mostFrequent string
	maxCount := 0
	for value, count := range counts {
		if count > maxCount {
			maxCount = count
			mostFrequent = value
		}
	}
	return mostFrequent
}

// getDetectorsByType 按类型获取检测器
func (s *IPQualityService) getDetectorsByType(detectorType detector.DetectorType) []detector.Detector {
	var result []detector.Detector
	for _, det := range s.detectorManager.GetEnabledDetectors() {
		if det.GetType() == detectorType {
			result = append(result, det)
		}
	}
	return result
}

// generateSummary 生成检测摘要
func (s *IPQualityService) generateSummary(result *IPQualityResult) {
	summary := result.Summary

	// 统计检测器数量
	totalDetectors := len(s.detectorManager.GetAllDetectors())
	successfulDetectors := 0
	failedDetectors := 0

	if result.GeolocationResult != nil {
		successfulDetectors++
		summary.Country = result.GeolocationResult.Country
		summary.City = result.GeolocationResult.City
		summary.ISP = result.GeolocationResult.ISP
		summary.IsVPN = result.GeolocationResult.IsVPN
		summary.IsProxy = result.GeolocationResult.IsProxy
		summary.IsTor = result.GeolocationResult.IsTor
	}

	if len(result.RiskResults) > 0 {
		successfulDetectors++
		// 使用第一个结果或合并逻辑来填充摘要
		for _, riskResult := range result.RiskResults {
			if riskResult.IsHighRisk {
				summary.IsHighRisk = true
			}
			if riskResult.RiskLevel != "" && riskResult.RiskLevel != "low" {
				summary.RiskLevel = riskResult.RiskLevel
			}
			summary.Recommendations = append(summary.Recommendations, riskResult.Recommendations...)
		}
	}

	if len(result.ServiceResults) > 0 {
		successfulDetectors++
		for _, serviceResult := range result.ServiceResults {
			if serviceResult.Status == "available" {
				summary.AvailableServices++
			} else if serviceResult.Status == "blocked" {
				summary.BlockedServices++
			}
		}
	}

	failedDetectors = totalDetectors - successfulDetectors

	summary.TotalDetectors = totalDetectors
	summary.SuccessfulDetectors = successfulDetectors
	summary.FailedDetectors = failedDetectors

	// 生成建议
	s.generateRecommendations(result, summary)
}

// generateRecommendations 生成建议
func (s *IPQualityService) generateRecommendations(result *IPQualityResult, summary *IPQualitySummary) {
	if summary.IsHighRisk {
		summary.Recommendations = append(summary.Recommendations, "High risk IP detected - consider additional verification")
	}

	if summary.IsVPN || summary.IsProxy || summary.IsTor {
		summary.Recommendations = append(summary.Recommendations, "Anonymous proxy detected - monitor for abuse")
	}

	if summary.BlockedServices > summary.AvailableServices {
		summary.Recommendations = append(summary.Recommendations, "Many services blocked - IP may be blacklisted")
	}

	if summary.SuccessfulDetectors < summary.TotalDetectors/2 {
		summary.Recommendations = append(summary.Recommendations, "Limited detection data - results may be incomplete")
	}
}

// calculateOverallScore 计算综合评分
func (s *IPQualityService) calculateOverallScore(result *IPQualityResult) {
	var score float64 = 50 // 默认中性评分

	// 基于地理位置调整评分
	if result.GeolocationResult != nil {
		if result.GeolocationResult.IsVPN || result.GeolocationResult.IsProxy {
			score -= 20
		}
		if result.GeolocationResult.IsTor {
			score -= 30
		}
		if result.GeolocationResult.IsHosting {
			score -= 10
		}
	}

	// 基于风险评估调整评分
	if len(result.RiskResults) > 0 {
		var totalRiskScore float64
		for _, riskResult := range result.RiskResults {
			totalRiskScore += riskResult.OverallScore
		}
		avgRiskScore := totalRiskScore / float64(len(result.RiskResults))
		if avgRiskScore > 0 {
			score -= (avgRiskScore / 100) * 40 // 风险评分影响40分
		}
	}

	// 基于服务解锁调整评分
	if len(result.ServiceResults) > 0 {
		availableCount := 0
		totalCount := len(result.ServiceResults)
		for _, serviceResult := range result.ServiceResults {
			if serviceResult.Status == "available" {
				availableCount++
			}
		}
		serviceScore := float64(availableCount) / float64(totalCount)
		score += serviceScore * 20 // 服务可用性影响20分
	}

	// 确保评分在0-100范围内
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	result.OverallScore = score
}

// determineOverallStatus 确定综合状态
func (s *IPQualityService) determineOverallStatus(result *IPQualityResult) {
	score := result.OverallScore

	if score >= 80 {
		result.OverallStatus = model.IPQualityStatusGood
	} else if score >= 60 {
		result.OverallStatus = model.IPQualityStatusFair
	} else if score >= 40 {
		result.OverallStatus = model.IPQualityStatusPoor
	} else if score >= 20 {
		result.OverallStatus = model.IPQualityStatusBad
	} else {
		result.OverallStatus = model.IPQualityStatusBanned
	}

	// 特殊情况处理
	if result.Summary.IsHighRisk {
		if result.OverallStatus == model.IPQualityStatusGood {
			result.OverallStatus = model.IPQualityStatusFair
		}
	}

	if result.Summary.IsTor {
		if result.OverallStatus == model.IPQualityStatusGood || result.OverallStatus == model.IPQualityStatusFair {
			result.OverallStatus = model.IPQualityStatusPoor
		}
	}
}

// Close 关闭服务
func (s *IPQualityService) Close() error {
	return nil
}
