package detector

import (
	"context"
	"time"

	"fmt"

	"passwall/internal/eventbus"
)

// DetectionError 检测错误
type DetectionError struct {
	Detector string
	IP       string
	Message  string
	Err      error
}

func (e *DetectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("detector '%s' failed to detect IP '%s': %s (%v)", e.Detector, e.IP, e.Message, e.Err)
	}
	return fmt.Sprintf("detector '%s' failed to detect IP '%s': %s", e.Detector, e.IP, e.Message)
}

func (e *DetectionError) Unwrap() error {
	return e.Err
}

// DetectorType 检测器类型
type DetectorType string

const (
	DetectorTypeGeolocation DetectorType = "geolocation" // 地理位置检测器
	DetectorTypeRisk        DetectorType = "risk"        // 风险评估检测器
	DetectorTypeService     DetectorType = "service"     // 应用服务检测器
)

// DetectorStatus 检测器状态
type DetectorStatus string

const (
	DetectorStatusUnknown     DetectorStatus = "unknown"     // 未知状态
	DetectorStatusAvailable   DetectorStatus = "available"   // 可用
	DetectorStatusUnavailable DetectorStatus = "unavailable" // 不可用
	DetectorStatusError       DetectorStatus = "error"       // 错误状态
)

// DetectorConfig 检测器配置
type DetectorConfig struct {
	Enabled      bool                   `json:"enabled"`
	Timeout      time.Duration          `json:"timeout"`
	RetryCount   int                    `json:"retry_count"`
	RetryDelay   time.Duration          `json:"retry_delay"`
	APIKey       string                 `json:"api_key,omitempty"`
	Endpoint     string                 `json:"endpoint,omitempty"`
	CustomParams map[string]interface{} `json:"custom_params,omitempty"`
}

// DetectorResult 检测结果
type DetectorResult struct {
	Success   bool                   `json:"success"`
	Error     error                  `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Provider  string                 `json:"provider"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GeolocationResult 地理位置检测结果
type GeolocationResult struct {
	Country      string  `json:"country"`
	CountryCode  string  `json:"country_code"`
	Region       string  `json:"region"`
	RegionCode   string  `json:"region_code"`
	City         string  `json:"city"`
	ZipCode      string  `json:"zip_code"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Timezone     string  `json:"timezone"`
	ISP          string  `json:"isp"`
	ASN          string  `json:"asn"`
	Organization string  `json:"organization"`
	IsVPN        bool    `json:"is_vpn"`
	IsProxy      bool    `json:"is_proxy"`
	IsHosting    bool    `json:"is_hosting"`
	IsTor        bool    `json:"is_tor"`
}

// RiskAssessmentResult 风险评估检测结果
type RiskAssessmentResult struct {
	OverallScore    float64    `json:"overall_score"`
	FraudScore      float64    `json:"fraud_score,omitempty"`
	SpamScore       float64    `json:"spam_score,omitempty"`
	BotScore        float64    `json:"bot_score,omitempty"`
	VPNProxyScore   float64    `json:"vpn_proxy_score,omitempty"`
	RiskLevel       string     `json:"risk_level"`
	IsHighRisk      bool       `json:"is_high_risk"`
	IsRecentAbuse   bool       `json:"is_recent_abuse"`
	LastReportedAt  *time.Time `json:"last_reported_at,omitempty"`
	RiskFactors     []string   `json:"risk_factors,omitempty"`
	Recommendations []string   `json:"recommendations,omitempty"`
}

// ServiceUnlockResult 应用服务解锁检测结果
type ServiceUnlockResult struct {
	ServiceName  string                 `json:"service_name"`
	ServiceType  string                 `json:"service_type"`
	Status       string                 `json:"status"`
	Region       string                 `json:"region,omitempty"`
	ContentType  string                 `json:"content_type,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Error        string                 `json:"error,omitempty"`
	ResponseTime time.Duration          `json:"response_time"`
}

// Detector 检测器接口
type Detector interface {
	GetType() DetectorType
	GetName() string
	GetVersion() string
	GetStatus() DetectorStatus
	GetConfig() DetectorConfig
	SetConfig(config DetectorConfig) error
	TestConnection(ctx context.Context) error
	Detect(ctx context.Context, ip string) (*DetectorResult, error)
	GetProvider() string
	IsEnabled() bool
	SetEnabled(enabled bool) error
}

// GeolocationDetector 地理位置检测器接口
type GeolocationDetector interface {
	Detector
	DetectGeolocation(ctx context.Context, ip string) (*GeolocationResult, error)
}

// RiskDetector 风险评估检测器接口
type RiskDetector interface {
	Detector
	DetectRisk(ctx context.Context, ip string) (*RiskAssessmentResult, error)
}

// ServiceDetector 应用服务检测器接口
type ServiceDetector interface {
	Detector
	DetectService(ctx context.Context, ip string) ([]*ServiceUnlockResult, error)
	GetSupportedServices() []string
}

// DetectorFactory 检测器工厂接口
type DetectorFactory interface {
	CreateDetector(detectorType string, config DetectorConfig) (Detector, error)
	GetAvailableDetectors() []string
	GetDetectorInfo(detectorType string) (*DetectorInfo, error)
}

// DetectorInfo 检测器信息
type DetectorInfo struct {
	Type           DetectorType           `json:"type"`
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Description    string                 `json:"description"`
	Author         string                 `json:"author"`
	Website        string                 `json:"website,omitempty"`
	License        string                 `json:"license"`
	Category       string                 `json:"category"`
	Tags           []string               `json:"tags"`
	ConfigSchema   map[string]interface{} `json:"config_schema"`
	RequiredConfig []string               `json:"required_config"`
}

// DetectorManager 检测器管理器接口
type DetectorManager interface {
	RegisterDetector(detector Detector) error
	UnregisterDetector(detectorType string) error
	GetDetector(detectorType string) (Detector, error)
	GetAllDetectors() []Detector
	GetEnabledDetectors() []Detector
	TestAllDetectors(ctx context.Context) map[string]error
	DetectAll(ctx context.Context, ip string) map[string]*DetectorResult
}

// BatchDetectOptions 批量检测选项
type BatchDetectOptions struct {
	Concurrency     int            `json:"concurrency"`
	Timeout         time.Duration  `json:"timeout"`
	RetryCount      int            `json:"retry_count"`
	RetryDelay      time.Duration  `json:"retry_delay"`
	DetectorTypes   []DetectorType `json:"detector_types,omitempty"`
	ContinueOnError bool           `json:"continue_on_error"`
}

// BatchDetectResult 批量检测结果
type BatchDetectResult struct {
	IP        string                     `json:"ip"`
	Results   map[string]*DetectorResult `json:"results"`
	Summary   *BatchDetectSummary        `json:"summary"`
	Duration  time.Duration              `json:"duration"`
	Timestamp time.Time                  `json:"timestamp"`
}

// BatchDetectSummary 批量检测摘要
type BatchDetectSummary struct {
	TotalDetectors  int           `json:"total_detectors"`
	SuccessCount    int           `json:"success_count"`
	FailureCount    int           `json:"failure_count"`
	AverageResponse time.Duration `json:"average_response"`
	MinLatency      time.Duration `json:"min_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	FastestDetector string        `json:"fastest_detector"`
	SlowestDetector string        `json:"slowest_detector"`
}

// DetectorEvent 检测器事件
type DetectorEvent struct {
	Type      string                 `json:"type"`
	Detector  string                 `json:"detector"`
	IP        string                 `json:"ip"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// 实现通用事件接口
func (de *DetectorEvent) GetType() string {
	return de.Type
}

func (de *DetectorEvent) GetTimestamp() time.Time {
	return de.Timestamp
}

func (de *DetectorEvent) GetData() map[string]interface{} {
	return de.Data
}

// DetectorEventHandler 检测器事件处理器接口
type DetectorEventHandler interface {
	HandleEvent(event *DetectorEvent) error
}

// EventBus 事件总线接口 (使用通用事件总线)
type EventBus interface {
	Publish(event eventbus.Event) error
	Subscribe(handler eventbus.EventHandler) error
	Unsubscribe(handler eventbus.EventHandler) error
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	RecordDetection(detectorType string, success bool, duration time.Duration)
	RecordError(detectorType string, errorType string)
	RecordLatency(detectorType string, latency time.Duration)
	GetMetrics(detectorType string) (*DetectorMetrics, error)
}

// DetectorMetrics 检测器指标
type DetectorMetrics struct {
	TotalDetections int64         `json:"total_detections"`
	SuccessCount    int64         `json:"success_count"`
	FailureCount    int64         `json:"failure_count"`
	AverageLatency  time.Duration `json:"average_latency"`
	MinLatency      time.Duration `json:"min_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	ErrorRate       float64       `json:"error_rate"`
	LastDetection   time.Time     `json:"last_detection"`
	Uptime          time.Duration `json:"uptime"`
}
