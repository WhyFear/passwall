package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"passwall/internal/detector"
)

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"` // streaming, ai, other
	TestURL        string            `json:"test_url"`
	ExpectedStatus int               `json:"expected_status"`
	SuccessPattern string            `json:"success_pattern"`
	BlockedPattern string            `json:"blocked_pattern"`
	Headers        map[string]string `json:"headers"`
	UserAgent      string            `json:"user_agent"`
	Timeout        time.Duration     `json:"timeout"`
}

// UniversalServiceDetector 通用应用服务检测器
type UniversalServiceDetector struct {
	config     detector.DetectorConfig
	httpClient *http.Client
	status     detector.DetectorStatus
	version    string
	services   map[string]ServiceConfig
}

// NewUniversalServiceDetector 创建通用应用服务检测器
func NewUniversalServiceDetector(config detector.DetectorConfig) (*UniversalServiceDetector, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.RetryCount == 0 {
		config.RetryCount = 2
	}

	detector := &UniversalServiceDetector{
		config:  config,
		status:  detector.DetectorStatusAvailable,
		version: "1.0.0",
		httpClient: &http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// 不跟随重定向，直接返回重定向响应
				return http.ErrUseLastResponse
			},
		},
		services: make(map[string]ServiceConfig),
	}

	// 初始化预设服务配置
	detector.initializeServices()

	return detector, nil
}

// GetType 获取检测器类型
func (d *UniversalServiceDetector) GetType() detector.DetectorType {
	return detector.DetectorTypeService
}

// GetName 获取检测器名称
func (d *UniversalServiceDetector) GetName() string {
	return "Universal Service Detector"
}

// GetVersion 获取检测器版本
func (d *UniversalServiceDetector) GetVersion() string {
	return d.version
}

// GetStatus 获取检测器状态
func (d *UniversalServiceDetector) GetStatus() detector.DetectorStatus {
	return d.status
}

// GetConfig 获取检测器配置
func (d *UniversalServiceDetector) GetConfig() detector.DetectorConfig {
	return d.config
}

// SetConfig 设置检测器配置
func (d *UniversalServiceDetector) SetConfig(config detector.DetectorConfig) error {
	d.config = config

	// 重新创建HTTP客户端
	d.httpClient = &http.Client{
		Timeout: config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return nil
}

// TestConnection 测试连接
func (d *UniversalServiceDetector) TestConnection(ctx context.Context) error {
	if !d.IsEnabled() {
		return fmt.Errorf("detector is not enabled")
	}

	// 测试一个简单的服务（如Netflix）
	if netflixConfig, exists := d.services["netflix"]; exists {
		result := d.testSingleService(ctx, "8.8.8.8", netflixConfig)
		if result.Status == "error" {
			d.status = detector.DetectorStatusError
			return fmt.Errorf("service detector connection test failed: %s", result.Error)
		}
	}

	d.status = detector.DetectorStatusAvailable
	return nil
}

// Detect 执行检测
func (d *UniversalServiceDetector) Detect(ctx context.Context, ip string) (*detector.DetectorResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	startTime := time.Now()

	// 执行服务解锁检测
	results, err := d.DetectService(ctx, ip)
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
		"total_services": len(results),
		"services":       results,
	}

	// 统计解锁状态
	var availableCount, blockedCount int
	servicesByType := make(map[string][]string)

	for _, result := range results {
		if result.Status == "available" {
			availableCount++
		} else if result.Status == "blocked" {
			blockedCount++
		}

		// 按类型分组
		if servicesByType[result.ServiceType] == nil {
			servicesByType[result.ServiceType] = make([]string, 0)
		}
		servicesByType[result.ServiceType] = append(servicesByType[result.ServiceType], result.ServiceName)
	}

	data["available_count"] = availableCount
	data["blocked_count"] = blockedCount
	data["services_by_type"] = servicesByType

	return &detector.DetectorResult{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		Provider:  d.GetProvider(),
		Metadata: map[string]interface{}{
			"duration":     time.Since(startTime),
			"test_count":   len(results),
			"success_rate": float64(availableCount) / float64(len(results)),
		},
	}, nil
}

// DetectService 执行服务解锁检测
func (d *UniversalServiceDetector) DetectService(ctx context.Context, ip string) ([]*detector.ServiceUnlockResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	var results []*detector.ServiceUnlockResult

	// 并发检测所有服务
	resultCh := make(chan *detector.ServiceUnlockResult, len(d.services))

	for serviceName, serviceConfig := range d.services {
		go func(name string, config ServiceConfig) {
			result := d.testSingleService(ctx, ip, config)
			result.ServiceName = name
			result.ServiceType = config.Type
			resultCh <- result
		}(serviceName, serviceConfig)
	}

	// 收集结果
	for i := 0; i < len(d.services); i++ {
		select {
		case result := <-resultCh:
			results = append(results, result)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return results, nil
}

// GetSupportedServices 获取支持的服务列表
func (d *UniversalServiceDetector) GetSupportedServices() []string {
	var services []string
	for serviceName := range d.services {
		services = append(services, serviceName)
	}
	return services
}

// GetProvider 获取提供商
func (d *UniversalServiceDetector) GetProvider() string {
	return "universal"
}

// IsEnabled 检查是否启用
func (d *UniversalServiceDetector) IsEnabled() bool {
	return d.config.Enabled
}

// SetEnabled 设置启用状态
func (d *UniversalServiceDetector) SetEnabled(enabled bool) error {
	d.config.Enabled = enabled
	return nil
}

// Close 关闭检测器
func (d *UniversalServiceDetector) Close() error {
	if d.httpClient != nil {
		d.httpClient.CloseIdleConnections()
	}
	return nil
}

// initializeServices 初始化预设服务配置
func (d *UniversalServiceDetector) initializeServices() {
	// Netflix 配置
	d.services["netflix"] = ServiceConfig{
		Name:           "Netflix",
		Type:           "streaming",
		TestURL:        "https://www.netflix.com/title/70143836",
		ExpectedStatus: 200,
		SuccessPattern: `"availability"`,
		BlockedPattern: `"error".*"blocked"`,
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.5",
			"Accept-Encoding": "gzip, deflate",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		Timeout:   15 * time.Second,
	}

	// Disney+ 配置
	d.services["disneyplus"] = ServiceConfig{
		Name:           "Disney+",
		Type:           "streaming",
		TestURL:        "https://www.disneyplus.com/",
		ExpectedStatus: 200,
		SuccessPattern: `"isAvailable":true`,
		BlockedPattern: `"isBlocked":true|"notAvailable"`,
		Headers: map[string]string{
			"Accept":          "application/json",
			"Accept-Language": "en-US,en;q=0.9",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		Timeout:   15 * time.Second,
	}

	// YouTube Premium 配置
	d.services["youtube"] = ServiceConfig{
		Name:           "YouTube Premium",
		Type:           "streaming",
		TestURL:        "https://www.youtube.com/premium",
		ExpectedStatus: 200,
		SuccessPattern: `"isAvailable":true`,
		BlockedPattern: `"countryCode".*"error"`,
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml",
			"Accept-Language": "en-US,en;q=0.9",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		Timeout:   10 * time.Second,
	}

	// ChatGPT 配置
	d.services["chatgpt"] = ServiceConfig{
		Name:           "ChatGPT",
		Type:           "ai",
		TestURL:        "https://chat.openai.com/api/auth/session",
		ExpectedStatus: 200,
		SuccessPattern: `"user"`,
		BlockedPattern: `"error".*"restricted"|"unavailable"`,
		Headers: map[string]string{
			"Accept":          "application/json",
			"Accept-Language": "en-US,en;q=0.9",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		Timeout:   20 * time.Second,
	}

	// Claude 配置
	d.services["claude"] = ServiceConfig{
		Name:           "Claude",
		Type:           "ai",
		TestURL:        "https://claude.ai/",
		ExpectedStatus: 200,
		SuccessPattern: `"available"`,
		BlockedPattern: `"blocked"|"restricted"`,
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml",
			"Accept-Language": "en-US,en;q=0.9",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		Timeout:   15 * time.Second,
	}
}

// testSingleService 测试单个服务
func (d *UniversalServiceDetector) testSingleService(ctx context.Context, ip string, config ServiceConfig) *detector.ServiceUnlockResult {
	startTime := time.Now()

	result := &detector.ServiceUnlockResult{
		ServiceName:  config.Name,
		ServiceType:  config.Type,
		Status:       "unknown",
		ResponseTime: 0,
		Details:      make(map[string]interface{}),
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", config.TestURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Status = "error"
		return result
	}

	// 设置请求头
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	if config.UserAgent != "" {
		req.Header.Set("User-Agent", config.UserAgent)
	}

	// 发送请求
	resp, err := d.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("request failed: %v", err)
		result.Status = "error"
		result.ResponseTime = time.Since(startTime)
		return result
	}
	defer resp.Body.Close()

	result.ResponseTime = time.Since(startTime)
	result.Details["status_code"] = resp.StatusCode
	result.Details["headers"] = resp.Header

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read response: %v", err)
		result.Status = "error"
		return result
	}

	bodyStr := string(body)
	result.Details["response_size"] = len(body)

	// 检查状态码
	if config.ExpectedStatus > 0 && resp.StatusCode != config.ExpectedStatus {
		result.Status = "blocked"
		result.Details["reason"] = "unexpected_status_code"
		return result
	}

	// 检查是否被阻止
	if config.BlockedPattern != "" {
		if matched, _ := regexp.MatchString(config.BlockedPattern, bodyStr); matched {
			result.Status = "blocked"
			result.Details["reason"] = "blocked_pattern_matched"
			return result
		}
	}

	// 检查是否可用
	if config.SuccessPattern != "" {
		if matched, _ := regexp.MatchString(config.SuccessPattern, bodyStr); matched {
			result.Status = "available"
			result.Details["reason"] = "success_pattern_matched"
			return result
		}
	}

	// 根据状态码判断
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = "available"
		result.Details["reason"] = "success_status_code"
	} else if resp.StatusCode == 403 || resp.StatusCode == 451 {
		result.Status = "blocked"
		result.Details["reason"] = "blocked_status_code"
	} else {
		result.Status = "unavailable"
		result.Details["reason"] = "other_status_code"
	}

	return result
}

// AddService 添加自定义服务配置
func (d *UniversalServiceDetector) AddService(name string, config ServiceConfig) {
	d.services[name] = config
}

// RemoveService 移除服务配置
func (d *UniversalServiceDetector) RemoveService(name string) {
	delete(d.services, name)
}

// GetServiceConfig 获取服务配置
func (d *UniversalServiceDetector) GetServiceConfig(name string) (ServiceConfig, bool) {
	config, exists := d.services[name]
	return config, exists
}
