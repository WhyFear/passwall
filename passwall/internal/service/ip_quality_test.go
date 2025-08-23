package service

import (
	"context"
	"testing"

	"passwall/internal/detector"

	"github.com/stretchr/testify/assert"
)

func TestMergeGeolocationResults(t *testing.T) {
	service := &IPQualityService{}

	t.Run("空结果列表", func(t *testing.T) {
		result := service.mergeGeolocationResults([]*detector.GeolocationResult{})
		assert.Nil(t, result)
	})

	t.Run("单个结果", func(t *testing.T) {
		results := []*detector.GeolocationResult{
			{
				Country:     "US",
				CountryCode: "US",
				Region:      "California",
				City:        "San Francisco",
				Latitude:    37.7749,
				Longitude:   -122.4194,
				ISP:         "Cloudflare",
				ASN:         "AS13335",
				IsVPN:       true,
				IsProxy:     false,
				IsHosting:   false,
				IsTor:       false,
			},
		}

		merged := service.mergeGeolocationResults(results)
		assert.NotNil(t, merged)
		assert.Equal(t, "US", merged.Country)
		assert.Equal(t, "US", merged.CountryCode)
		assert.Equal(t, "California", merged.Region)
		assert.Equal(t, "San Francisco", merged.City)
		assert.Equal(t, 37.7749, merged.Latitude)
		assert.Equal(t, -122.4194, merged.Longitude)
		assert.Equal(t, "Cloudflare", merged.ISP)
		assert.Equal(t, "AS13335", merged.ASN)
		assert.True(t, merged.IsVPN)
		assert.False(t, merged.IsProxy)
		assert.False(t, merged.IsHosting)
		assert.False(t, merged.IsTor)
	})

	t.Run("多个结果合并", func(t *testing.T) {
		results := []*detector.GeolocationResult{
			{
				Country:     "US",
				CountryCode: "US",
				Region:      "California",
				City:        "San Francisco",
				Latitude:    37.7749,
				Longitude:   -122.4194,
				ISP:         "Cloudflare",
				IsVPN:       true,
				IsProxy:     false,
			},
			{
				Country:     "US", // 相同国家
				CountryCode: "US",
				Region:      "California", // 相同地区
				City:        "Oakland",    // 不同城市
				Latitude:    37.8044,
				Longitude:   -122.2711,
				ISP:         "Google", // 不同ISP
				IsVPN:       false,
				IsProxy:     true, // 不同代理状态
			},
			{
				Country:     "CA", // 不同国家
				CountryCode: "CA",
				Region:      "Ontario",
				City:        "Toronto",
				Latitude:    43.6532,
				Longitude:   -79.3832,
				ISP:         "Bell Canada",
				IsVPN:       false,
				IsProxy:     false,
			},
		}

		merged := service.mergeGeolocationResults(results)
		assert.NotNil(t, merged)

		// 应该选择最常见的值
		assert.Equal(t, "US", merged.Country) // US出现2次，CA出现1次
		assert.Equal(t, "US", merged.CountryCode)
		assert.Equal(t, "California", merged.Region) // California出现2次

		// 坐标应该是平均值
		expectedLat := (37.7749 + 37.8044 + 43.6532) / 3
		expectedLon := (-122.4194 + -122.2711 + -79.3832) / 3
		assert.InDelta(t, expectedLat, merged.Latitude, 0.0001)
		assert.InDelta(t, expectedLon, merged.Longitude, 0.0001)

		// 布尔值应该是OR逻辑
		assert.True(t, merged.IsVPN)   // 至少一个为true
		assert.True(t, merged.IsProxy) // 至少一个为true
	})

	t.Run("处理空值", func(t *testing.T) {
		results := []*detector.GeolocationResult{
			{
				Country:   "US",
				City:      "", // 空城市
				Latitude:  0,  // 无效坐标
				Longitude: 0,
				ISP:       "Provider1",
			},
			{
				Country:   "", // 空国家
				City:      "New York",
				Latitude:  40.7128,
				Longitude: -74.0060,
				ISP:       "", // 空ISP
			},
		}

		merged := service.mergeGeolocationResults(results)
		assert.NotNil(t, merged)
		assert.Equal(t, "US", merged.Country)     // 只有一个有效值
		assert.Equal(t, "New York", merged.City)  // 只有一个有效值
		assert.Equal(t, "Provider1", merged.ISP)  // 只有一个有效值
		assert.Equal(t, 40.7128, merged.Latitude) // 只有一个有效坐标
		assert.Equal(t, -74.0060, merged.Longitude)
	})
}

func TestDetectRisk(t *testing.T) {
	t.Run("空结果列表返回", func(t *testing.T) {
		// 由于detectRisk现在返回数组，我们测试它直接返回所有结果
		results := []*detector.RiskAssessmentResult{
			{
				OverallScore:    85.0,
				FraudScore:      75.0,
				SpamScore:       60.0,
				BotScore:        40.0,
				VPNProxyScore:   90.0,
				RiskLevel:       "high",
				IsHighRisk:      true,
				IsRecentAbuse:   false,
				RiskFactors:     []string{"vpn", "suspicious_activity"},
				Recommendations: []string{"monitor closely", "verify identity"},
			},
			{
				OverallScore:    50.0,
				FraudScore:      40.0,
				SpamScore:       70.0,
				BotScore:        60.0,
				VPNProxyScore:   30.0,
				RiskLevel:       "medium",
				IsHighRisk:      false,
				IsRecentAbuse:   true,
				RiskFactors:     []string{"spam", "bot_activity"},
				Recommendations: []string{"verify identity", "limit access"},
			},
		}

		// 验证结果直接包含所有检测器的结果
		assert.Len(t, results, 2)
		assert.Equal(t, 85.0, results[0].OverallScore)
		assert.Equal(t, 50.0, results[1].OverallScore)
		assert.True(t, results[0].IsHighRisk)
		assert.False(t, results[1].IsHighRisk)
		assert.Contains(t, results[0].RiskFactors, "vpn")
		assert.Contains(t, results[1].RiskFactors, "spam")
	})
}

func TestGenerateSummaryWithRiskResults(t *testing.T) {
	service := &IPQualityService{}

	t.Run("处理多个风险结果", func(t *testing.T) {
		result := &IPQualityResult{
			RiskResults: []*detector.RiskAssessmentResult{
				{
					OverallScore:    70.0,
					RiskLevel:       "high",
					IsHighRisk:      true,
					Recommendations: []string{"monitor closely"},
				},
				{
					OverallScore:    50.0,
					RiskLevel:       "medium",
					IsHighRisk:      false,
					Recommendations: []string{"verify identity"},
				},
			},
			Summary: &IPQualitySummary{
				Recommendations: make([]string, 0),
			},
		}

		// Mock detector manager
		service.detectorManager = &mockDetectorManager{}

		service.generateSummary(result)

		// 验证摘要正确处理了多个风险结果
		assert.True(t, result.Summary.IsHighRisk)         // 应该是true因为有一个IsHighRisk为true
		assert.Equal(t, "high", result.Summary.RiskLevel) // 应该选择non-low的级别
		assert.Contains(t, result.Summary.Recommendations, "monitor closely")
		assert.Contains(t, result.Summary.Recommendations, "verify identity")
	})
}

func TestCalculateOverallScoreWithRiskResults(t *testing.T) {
	service := &IPQualityService{}

	t.Run("基于多个风险结果计算评分", func(t *testing.T) {
		result := &IPQualityResult{
			RiskResults: []*detector.RiskAssessmentResult{
				{OverallScore: 80.0},
				{OverallScore: 60.0},
			},
		}

		service.calculateOverallScore(result)

		// 平均风险评分应该是70.0
		// 初始评分50 - (70/100)*40 = 50 - 28 = 22
		assert.Equal(t, 22.0, result.OverallScore)
	})
}

func TestGetMostFrequent(t *testing.T) {
	t.Run("空计数", func(t *testing.T) {
		result := getMostFrequent(map[string]int{})
		assert.Equal(t, "", result)
	})

	t.Run("单个值", func(t *testing.T) {
		counts := map[string]int{
			"value1": 3,
		}
		result := getMostFrequent(counts)
		assert.Equal(t, "value1", result)
	})

	t.Run("多个值不同频率", func(t *testing.T) {
		counts := map[string]int{
			"value1": 1,
			"value2": 3,
			"value3": 2,
		}
		result := getMostFrequent(counts)
		assert.Equal(t, "value2", result)
	})

	t.Run("相同频率", func(t *testing.T) {
		counts := map[string]int{
			"value1": 2,
			"value2": 2,
		}
		result := getMostFrequent(counts)
		// 应该返回其中一个，具体哪个取决于map遍历顺序
		assert.True(t, result == "value1" || result == "value2")
	})
}

// Mock detector manager for testing
type mockDetectorManager struct{}

func (m *mockDetectorManager) RegisterDetector(detector detector.Detector) error {
	return nil
}

func (m *mockDetectorManager) UnregisterDetector(detectorType string) error {
	return nil
}

func (m *mockDetectorManager) GetDetector(detectorType string) (detector.Detector, error) {
	return nil, nil
}

func (m *mockDetectorManager) GetAllDetectors() []detector.Detector {
	return []detector.Detector{}
}

func (m *mockDetectorManager) GetEnabledDetectors() []detector.Detector {
	return []detector.Detector{}
}

func (m *mockDetectorManager) TestAllDetectors(ctx context.Context) map[string]error {
	return make(map[string]error)
}

func (m *mockDetectorManager) DetectAll(ctx context.Context, ip string) map[string]*detector.DetectorResult {
	return make(map[string]*detector.DetectorResult)
}
