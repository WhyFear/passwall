package geolocation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"passwall/internal/detector"
)

// IPInfoDetector IPInfo地理位置检测器
type IPInfoDetector struct {
	config     detector.DetectorConfig
	client     *http.Client
	status     detector.DetectorStatus
	version    string
	httpClient *http.Client
}

// IPInfoResponse IPInfo API响应结构
type IPInfoResponse struct {
	IP       string         `json:"ip"`
	Hostname string         `json:"hostname"`
	City     string         `json:"city"`
	Region   string         `json:"region"`
	Country  string         `json:"country"`
	Loc      string         `json:"loc"`      // 纬度,经度
	Org      string         `json:"org"`      // 组织
	Postal   string         `json:"postal"`   // 邮政编码
	Timezone string         `json:"timezone"` // 时区
	ASN      *IPInfoASN     `json:"asn,omitempty"`
	Company  *IPInfoCompany `json:"company,omitempty"`
	Privacy  *IPInfoPrivacy `json:"privacy,omitempty"`
	Abuse    *IPInfoAbuse   `json:"abuse,omitempty"`
	Domains  *IPInfoDomains `json:"domains,omitempty"`
}

// IPInfoASN ASN信息
type IPInfoASN struct {
	ASN         string `json:"asn"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	Allocated   string `json:"allocated"`
	Registry    string `json:"registry"`
	Domain      string `json:"domain"`
	Type        string `json:"type"`
	Route       string `json:"route"`
	DomainCount int    `json:"domain_count"`
}

// IPInfoCompany 公司信息
type IPInfoCompany struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Type   string `json:"type"`
}

// IPInfoPrivacy 隐私信息
type IPInfoPrivacy struct {
	VPN     bool `json:"vpn"`
	Proxy   bool `json:"proxy"`
	Tor     bool `json:"tor"`
	Relay   bool `json:"relay"`
	Hosting bool `json:"hosting"`
	Service bool `json:"service"`
	Spider  bool `json:"spider"`
	Scanner bool `json:"scanner"`
	Bot     bool `json:"bot"`
}

// IPInfoAbuse 滥用信息
type IPInfoAbuse struct {
	Address string `json:"address"`
	Country string `json:"country"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Network string `json:"network"`
	Phone   string `json:"phone"`
}

// IPInfoDomains 域名信息
type IPInfoDomains struct {
	IP      string   `json:"ip"`
	Total   int      `json:"total"`
	Domains []string `json:"domains"`
}

// NewIPInfoDetector 创建新的IPInfo检测器
func NewIPInfoDetector(config detector.DetectorConfig) (*IPInfoDetector, error) {
	if config.CustomParams == nil {
		config.CustomParams = make(map[string]interface{})
	}

	d := &IPInfoDetector{
		config:  config,
		status:  detector.DetectorStatusUnknown,
		version: "1.0.0",
	}

	// 验证必需配置
	if config.APIKey == "" {
		return nil, fmt.Errorf("api_key is required for IPinfo detector")
	}

	// 设置默认端点
	if config.Endpoint == "" {
		config.Endpoint = "https://ipinfo.io"
	}

	// 创建HTTP客户端
	d.httpClient = &http.Client{
		Timeout: config.Timeout,
	}

	// 测试连接
	if err := d.TestConnection(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize IPInfo detector: %w", err)
	}

	return d, nil
}

// GetType 获取检测器类型
func (d *IPInfoDetector) GetType() detector.DetectorType {
	return detector.DetectorTypeGeolocation
}

// GetName 获取检测器名称
func (d *IPInfoDetector) GetName() string {
	return "IPInfo"
}

// GetVersion 获取检测器版本
func (d *IPInfoDetector) GetVersion() string {
	return d.version
}

// GetStatus 获取检测器状态
func (d *IPInfoDetector) GetStatus() detector.DetectorStatus {
	return d.status
}

// GetConfig 获取检测器配置
func (d *IPInfoDetector) GetConfig() detector.DetectorConfig {
	return d.config
}

// SetConfig 设置检测器配置
func (d *IPInfoDetector) SetConfig(config detector.DetectorConfig) error {
	d.config = config

	// 更新HTTP客户端超时
	if d.httpClient != nil {
		d.httpClient.Timeout = config.Timeout
	}

	return nil
}

// TestConnection 测试连接
func (d *IPInfoDetector) TestConnection(ctx context.Context) error {
	if !d.IsEnabled() {
		return fmt.Errorf("detector is not enabled")
	}

	// 使用测试IP进行连接测试
	testIP := "8.8.8.8"
	_, err := d.fetchIPInfo(ctx, testIP)
	if err != nil {
		d.status = detector.DetectorStatusError
		return fmt.Errorf("failed to test IPInfo connection: %w", err)
	}

	d.status = detector.DetectorStatusAvailable
	return nil
}

// Detect 执行检测
func (d *IPInfoDetector) Detect(ctx context.Context, ip string) (*detector.DetectorResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	startTime := time.Now()

	// 执行地理位置检测
	result, err := d.DetectGeolocation(ctx, ip)
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
		"country":      result.Country,
		"country_code": result.CountryCode,
		"region":       result.Region,
		"region_code":  result.RegionCode,
		"city":         result.City,
		"zip_code":     result.ZipCode,
		"latitude":     result.Latitude,
		"longitude":    result.Longitude,
		"timezone":     result.Timezone,
		"isp":          result.ISP,
		"asn":          result.ASN,
		"organization": result.Organization,
		"is_vpn":       result.IsVPN,
		"is_proxy":     result.IsProxy,
		"is_hosting":   result.IsHosting,
		"is_tor":       result.IsTor,
	}

	return &detector.DetectorResult{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		Provider:  d.GetProvider(),
		Metadata: map[string]interface{}{
			"duration": time.Since(startTime),
		},
	}, nil
}

// DetectGeolocation 执行地理位置检测
func (d *IPInfoDetector) DetectGeolocation(ctx context.Context, ip string) (*detector.GeolocationResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	// 获取IP信息
	response, err := d.fetchIPInfo(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IP info: %w", err)
	}

	// 解析位置信息
	var latitude, longitude float64
	if response.Loc != "" {
		if _, err := fmt.Sscanf(response.Loc, "%f,%f", &latitude, &longitude); err != nil {
			// 解析失败，设置为0
			latitude, longitude = 0, 0
		}
	}

	// 构建结果
	result := &detector.GeolocationResult{
		Country:     response.Country,
		CountryCode: response.Country,
		Region:      response.Region,
		City:        response.City,
		ZipCode:     response.Postal,
		Latitude:    latitude,
		Longitude:   longitude,
		Timezone:    response.Timezone,
		IsVPN:       false,
		IsProxy:     false,
		IsHosting:   false,
		IsTor:       false,
	}

	// 设置组织信息
	if response.Org != "" {
		result.ISP = response.Org
		result.Organization = response.Org
	}

	// 设置ASN信息
	if response.ASN != nil {
		result.ASN = response.ASN.ASN
		if response.ASN.Name != "" {
			result.Organization = response.ASN.Name
		}
	}

	// 设置公司信息
	if response.Company != nil {
		if response.Company.Name != "" {
			result.Organization = response.Company.Name
		}
	}

	// 设置隐私信息
	if response.Privacy != nil {
		result.IsVPN = response.Privacy.VPN
		result.IsProxy = response.Privacy.Proxy
		result.IsHosting = response.Privacy.Hosting
		result.IsTor = response.Privacy.Tor
	}

	return result, nil
}

// fetchIPInfo 获取IP信息
func (d *IPInfoDetector) fetchIPInfo(ctx context.Context, ip string) (*IPInfoResponse, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	// 构建请求URL
	url := fmt.Sprintf("%s/%s/json?token=%s", d.config.Endpoint, ip, d.config.APIKey)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 发送请求
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response IPInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// GetProvider 获取提供商
func (d *IPInfoDetector) GetProvider() string {
	return "ipinfo"
}

// IsEnabled 检查是否启用
func (d *IPInfoDetector) IsEnabled() bool {
	return d.config.Enabled
}

// SetEnabled 设置启用状态
func (d *IPInfoDetector) SetEnabled(enabled bool) error {
	d.config.Enabled = enabled
	return nil
}

// GetRateLimitInfo 获取速率限制信息
func (d *IPInfoDetector) GetRateLimitInfo(ctx context.Context) (*IPInfoRateLimit, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	// IPInfo的速率限制信息通常在响应头中
	// 这里发送一个测试请求来获取速率限制信息
	url := fmt.Sprintf("%s/8.8.8.8/json?token=%s", d.config.Endpoint, d.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 解析速率限制头
	rateLimit := &IPInfoRateLimit{
		Remaining: parseRateLimitHeader(resp.Header.Get("X-RateLimit-Remaining")),
		Reset:     parseRateLimitReset(resp.Header.Get("X-RateLimit-Reset")),
		Limit:     parseRateLimitHeader(resp.Header.Get("X-RateLimit-Limit")),
	}

	return rateLimit, nil
}

// IPInfoRateLimit 速率限制信息
type IPInfoRateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// parseRateLimitHeader 解析速率限制头
func parseRateLimitHeader(header string) int {
	if header == "" {
		return 0
	}
	var limit int
	if _, err := fmt.Sscanf(header, "%d", &limit); err != nil {
		return 0
	}
	return limit
}

// parseRateLimitReset 解析速率限制重置时间
func parseRateLimitReset(header string) time.Time {
	if header == "" {
		return time.Time{}
	}
	var timestamp int64
	if _, err := fmt.Sscanf(header, "%d", &timestamp); err != nil {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// ValidateAPIKey 验证API密钥
func (d *IPInfoDetector) ValidateAPIKey(ctx context.Context) error {
	if !d.IsEnabled() {
		return fmt.Errorf("detector is not enabled")
	}

	// 使用测试IP验证API密钥
	_, err := d.fetchIPInfo(ctx, "8.8.8.8")
	if err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	return nil
}
