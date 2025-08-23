package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"passwall/internal/detector"
)

// ScamalyticsResponse Scamalytics API响应结构
type ScamalyticsResponse struct {
	IP           string   `json:"ip"`
	Risk         float64  `json:"risk"`
	IsVPN        bool     `json:"is_vpn"`
	IsProxy      bool     `json:"is_proxy"`
	IsTor        bool     `json:"is_tor"`
	IsDatacenter bool     `json:"is_datacenter"`
	ISP          string   `json:"isp"`
	Organization string   `json:"organization"`
	ASNumber     string   `json:"as_number"`
	ASName       string   `json:"as_name"`
	Country      string   `json:"country"`
	City         string   `json:"city"`
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	LastSeen     string   `json:"last_seen"`
	ThreatTypes  []string `json:"threat_types"`
	ThreatScore  float64  `json:"threat_score"`
}

// ScamalyticsDetector Scamalytics风险评估检测器
type ScamalyticsDetector struct {
	config     detector.DetectorConfig
	httpClient *http.Client
	status     detector.DetectorStatus
	version    string
}

// NewScamalyticsDetector 创建Scamalytics检测器
func NewScamalyticsDetector(config detector.DetectorConfig) (*ScamalyticsDetector, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Scamalytics detector")
	}

	if config.Endpoint == "" {
		config.Endpoint = "https://scamalytics.com/api"
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.RetryCount == 0 {
		config.RetryCount = 3
	}

	return &ScamalyticsDetector{
		config:  config,
		status:  detector.DetectorStatusAvailable,
		version: "1.0.0",
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// GetType 获取检测器类型
func (d *ScamalyticsDetector) GetType() detector.DetectorType {
	return detector.DetectorTypeRisk
}

// GetName 获取检测器名称
func (d *ScamalyticsDetector) GetName() string {
	return "Scamalytics"
}

// GetVersion 获取检测器版本
func (d *ScamalyticsDetector) GetVersion() string {
	return d.version
}

// GetStatus 获取检测器状态
func (d *ScamalyticsDetector) GetStatus() detector.DetectorStatus {
	return d.status
}

// GetConfig 获取检测器配置
func (d *ScamalyticsDetector) GetConfig() detector.DetectorConfig {
	return d.config
}

// SetConfig 设置检测器配置
func (d *ScamalyticsDetector) SetConfig(config detector.DetectorConfig) error {
	d.config = config

	// 重新创建HTTP客户端
	d.httpClient = &http.Client{
		Timeout: config.Timeout,
	}

	return nil
}

// TestConnection 测试连接
func (d *ScamalyticsDetector) TestConnection(ctx context.Context) error {
	if !d.IsEnabled() {
		return fmt.Errorf("detector is not enabled")
	}

	// 使用测试IP进行健康检查
	testIP := "8.8.8.8"
	_, err := d.callScamalyticsAPI(ctx, testIP)
	if err != nil {
		d.status = detector.DetectorStatusError
		return fmt.Errorf("Scamalytics connection test failed: %w", err)
	}

	d.status = detector.DetectorStatusAvailable
	return nil
}

// Detect 执行检测
func (d *ScamalyticsDetector) Detect(ctx context.Context, ip string) (*detector.DetectorResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	startTime := time.Now()

	// 调用Scamalytics API
	response, err := d.callScamalyticsAPI(ctx, ip)
	if err != nil {
		return &detector.DetectorResult{
			Success:   false,
			Error:     err,
			Timestamp: time.Now(),
			Provider:  d.GetProvider(),
		}, nil
	}

	// 执行风险评估检测
	riskResult, err := d.DetectRisk(ctx, ip)
	if err != nil {
		return &detector.DetectorResult{
			Success:   false,
			Error:     err,
			Timestamp: time.Now(),
			Provider:  d.GetProvider(),
		}, nil
	}

	// 转换为通用格式
	data := map[string]interface{}{
		"risk_score":      riskResult.OverallScore,
		"fraud_score":     riskResult.FraudScore,
		"spam_score":      riskResult.SpamScore,
		"bot_score":       riskResult.BotScore,
		"vpn_proxy_score": riskResult.VPNProxyScore,
		"risk_level":      riskResult.RiskLevel,
		"is_high_risk":    riskResult.IsHighRisk,
		"is_recent_abuse": riskResult.IsRecentAbuse,
		"risk_factors":    riskResult.RiskFactors,
		"recommendations": riskResult.Recommendations,
		"threat_types":    response.ThreatTypes,
		"is_vpn":          response.IsVPN,
		"is_proxy":        response.IsProxy,
		"is_tor":          response.IsTor,
		"is_datacenter":   response.IsDatacenter,
		"isp":             response.ISP,
		"organization":    response.Organization,
		"country":         response.Country,
		"city":            response.City,
	}

	return &detector.DetectorResult{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		Provider:  d.GetProvider(),
		Metadata: map[string]interface{}{
			"duration":     time.Since(startTime),
			"threat_score": response.ThreatScore,
			"last_seen":    response.LastSeen,
			"as_number":    response.ASNumber,
			"as_name":      response.ASName,
		},
	}, nil
}

// DetectRisk 执行风险评估检测
func (d *ScamalyticsDetector) DetectRisk(ctx context.Context, ip string) (*detector.RiskAssessmentResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	// 调用Scamalytics API
	response, err := d.callScamalyticsAPI(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to call Scamalytics API: %w", err)
	}

	// 转换为风险评估结果
	return d.convertToRiskResult(response), nil
}

// GetProvider 获取提供商
func (d *ScamalyticsDetector) GetProvider() string {
	return "scamalytics"
}

// IsEnabled 检查是否启用
func (d *ScamalyticsDetector) IsEnabled() bool {
	return d.config.Enabled
}

// SetEnabled 设置启用状态
func (d *ScamalyticsDetector) SetEnabled(enabled bool) error {
	d.config.Enabled = enabled
	return nil
}

// Close 关闭检测器
func (d *ScamalyticsDetector) Close() error {
	if d.httpClient != nil {
		d.httpClient.CloseIdleConnections()
	}
	return nil
}

// callScamalyticsAPI 调用Scamalytics API
func (d *ScamalyticsDetector) callScamalyticsAPI(ctx context.Context, ip string) (*ScamalyticsResponse, error) {
	url := fmt.Sprintf("%s/ip/%s", d.config.Endpoint, ip)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+d.config.APIKey)
	req.Header.Set("User-Agent", "Passwall/1.0")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	// 解析响应
	var response ScamalyticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// convertToRiskResult 转换为风险评估结果
func (d *ScamalyticsDetector) convertToRiskResult(response *ScamalyticsResponse) *detector.RiskAssessmentResult {
	result := &detector.RiskAssessmentResult{
		OverallScore:  response.Risk,
		FraudScore:    response.ThreatScore,
		VPNProxyScore: 0,
		RiskLevel:     d.calculateRiskLevel(response.Risk),
		IsHighRisk:    response.Risk > 70,
		IsRecentAbuse: len(response.ThreatTypes) > 0,
		RiskFactors:   response.ThreatTypes,
	}

	// 计算VPN/代理评分
	if response.IsVPN {
		result.VPNProxyScore = 60
	}
	if response.IsProxy {
		result.VPNProxyScore = max(result.VPNProxyScore, 80)
	}
	if response.IsTor {
		result.VPNProxyScore = max(result.VPNProxyScore, 90)
	}

	// 添加建议
	result.Recommendations = d.generateRecommendations(response)

	return result
}

// calculateRiskLevel 计算风险等级
func (d *ScamalyticsDetector) calculateRiskLevel(score float64) string {
	if score >= 80 {
		return "critical"
	} else if score >= 60 {
		return "high"
	} else if score >= 40 {
		return "medium"
	} else if score >= 20 {
		return "low"
	}
	return "unknown"
}

// generateRecommendations 生成建议
func (d *ScamalyticsDetector) generateRecommendations(response *ScamalyticsResponse) []string {
	var recommendations []string

	if response.Risk > 70 {
		recommendations = append(recommendations, "High risk IP - consider blocking")
	}
	if response.IsProxy || response.IsVPN || response.IsTor {
		recommendations = append(recommendations, "Anonymous proxy detected - verify user identity")
	}
	if response.IsDatacenter {
		recommendations = append(recommendations, "Datacenter IP - may indicate automated access")
	}
	if len(response.ThreatTypes) > 0 {
		recommendations = append(recommendations, "Threat types detected: "+fmt.Sprintf("%v", response.ThreatTypes))
	}

	return recommendations
}

// max 返回两个float64的最大值
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
