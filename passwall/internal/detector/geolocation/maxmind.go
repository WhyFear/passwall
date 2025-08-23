package geolocation

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/oschwald/geoip2-golang"

	"passwall/internal/detector"
)

// MaxmindDetector Maxmind地理位置检测器
type MaxmindDetector struct {
	config      detector.DetectorConfig
	db          *geoip2.Reader
	status      detector.DetectorStatus
	version     string
	initialized bool
}

// NewMaxmindDetector 创建新的Maxmind检测器
func NewMaxmindDetector(config detector.DetectorConfig) (*MaxmindDetector, error) {
	if config.CustomParams == nil {
		config.CustomParams = make(map[string]interface{})
	}

	d := &MaxmindDetector{
		config:  config,
		status:  detector.DetectorStatusUnknown,
		version: "1.0.0",
	}

	// 验证必需配置
	dbPath, ok := config.CustomParams["database_path"].(string)
	if !ok || dbPath == "" {
		return nil, fmt.Errorf("database_path is required for Maxmind detector")
	}

	// 初始化数据库
	if err := d.initializeDatabase(dbPath); err != nil {
		return nil, fmt.Errorf("failed to initialize Maxmind database: %w", err)
	}

	return d, nil
}

// initializeDatabase 初始化数据库
func (d *MaxmindDetector) initializeDatabase(dbPath string) error {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open Maxmind database: %w", err)
	}

	d.db = db
	d.initialized = true
	d.status = detector.DetectorStatusAvailable
	return nil
}

// GetType 获取检测器类型
func (d *MaxmindDetector) GetType() detector.DetectorType {
	return detector.DetectorTypeGeolocation
}

// GetName 获取检测器名称
func (d *MaxmindDetector) GetName() string {
	return "Maxmind GeoIP"
}

// GetVersion 获取检测器版本
func (d *MaxmindDetector) GetVersion() string {
	return d.version
}

// GetStatus 获取检测器状态
func (d *MaxmindDetector) GetStatus() detector.DetectorStatus {
	return d.status
}

// GetConfig 获取检测器配置
func (d *MaxmindDetector) GetConfig() detector.DetectorConfig {
	return d.config
}

// SetConfig 设置检测器配置
func (d *MaxmindDetector) SetConfig(config detector.DetectorConfig) error {
	d.config = config

	// 如果数据库路径改变，重新初始化
	if dbPath, ok := config.CustomParams["database_path"].(string); ok && dbPath != "" {
		if d.db != nil {
			_ = d.db.Close()
		}
		return d.initializeDatabase(dbPath)
	}

	return nil
}

// TestConnection 测试连接
func (d *MaxmindDetector) TestConnection(ctx context.Context) error {
	if !d.initialized || d.db == nil {
		return fmt.Errorf("Maxmind database not initialized")
	}

	// 使用测试IP查询数据库
	testIP := net.ParseIP("8.8.8.8")
	if testIP == nil {
		return fmt.Errorf("invalid test IP address")
	}

	_, err := d.db.City(testIP)
	if err != nil {
		d.status = detector.DetectorStatusError
		return fmt.Errorf("failed to query Maxmind database: %w", err)
	}

	d.status = detector.DetectorStatusAvailable
	return nil
}

// Detect 执行检测
func (d *MaxmindDetector) Detect(ctx context.Context, ip string) (*detector.DetectorResult, error) {
	if !d.IsEnabled() {
		return nil, fmt.Errorf("detector is not enabled")
	}

	startTime := time.Now()

	// 解析IP地址
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return &detector.DetectorResult{
			Success:   false,
			Error:     fmt.Errorf("invalid IP address: %s", ip),
			Timestamp: time.Now(),
			Provider:  d.GetProvider(),
		}, nil
	}

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
func (d *MaxmindDetector) DetectGeolocation(ctx context.Context, ip string) (*detector.GeolocationResult, error) {
	if !d.initialized || d.db == nil {
		return nil, fmt.Errorf("Maxmind database not initialized")
	}

	// 解析IP地址
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	// 查询城市信息
	city, err := d.db.City(ipAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to query city information: %w", err)
	}

	// Note: ISP和Anonymous IP查询需要单独的数据库文件，当前仅使用City数据库
	var ispResult *geoip2.ISP // 暂时不使用
	_ = ispResult             // 避免未使用变量警告

	// 构建结果
	result := &detector.GeolocationResult{
		Country:     city.Country.Names["en"],
		CountryCode: city.Country.IsoCode,
		City:        city.City.Names["en"],
		Latitude:    city.Location.Latitude,
		Longitude:   city.Location.Longitude,
		Timezone:    city.Location.TimeZone,
	}

	// 设置区域信息
	if len(city.Subdivisions) > 0 {
		result.Region = city.Subdivisions[0].Names["en"]
		result.RegionCode = city.Subdivisions[0].IsoCode
	}

	// 设置邮政编码
	if city.Postal.Code != "" {
		result.ZipCode = city.Postal.Code
	}

	// 设置ISP信息
	if ispResult != nil {
		result.ISP = ispResult.ISP
		result.ASN = fmt.Sprintf("%d", ispResult.AutonomousSystemNumber)
		result.Organization = ispResult.AutonomousSystemOrganization
	}

	// 设置匿名代理信息（暂时设置为默认值）
	result.IsVPN = false
	result.IsProxy = false
	result.IsHosting = false
	result.IsTor = false

	return result, nil
}

// GetProvider 获取提供商
func (d *MaxmindDetector) GetProvider() string {
	return "maxmind"
}

// IsEnabled 检查是否启用
func (d *MaxmindDetector) IsEnabled() bool {
	return d.config.Enabled
}

// SetEnabled 设置启用状态
func (d *MaxmindDetector) SetEnabled(enabled bool) error {
	d.config.Enabled = enabled
	return nil
}

// Close 关闭检测器
func (d *MaxmindDetector) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// MaxmindDatabaseInfo Maxmind数据库信息
type MaxmindDatabaseInfo struct {
	DatabaseType string    `json:"database_type"`
	BinaryFormat uint16    `json:"binary_format"`
	BuildEpoch   time.Time `json:"build_epoch"`
	Description  string    `json:"description"`
	IPVersion    string    `json:"ip_version"`
	NodeCount    uint32    `json:"node_count"`
	RecordSize   uint16    `json:"record_size"`
}

// GetDatabaseInfo 获取数据库信息
func (d *MaxmindDetector) GetDatabaseInfo() (*MaxmindDatabaseInfo, error) {
	if !d.initialized || d.db == nil {
		return nil, fmt.Errorf("Maxmind database not initialized")
	}

	metadata := d.db.Metadata()

	return &MaxmindDatabaseInfo{
		DatabaseType: metadata.DatabaseType,
		BinaryFormat: uint16(metadata.BinaryFormatMajorVersion),
		BuildEpoch:   time.Unix(int64(metadata.BuildEpoch), 0),
		Description:  metadata.Description["en"],
		IPVersion:    fmt.Sprintf("%d", metadata.IPVersion),
		NodeCount:    uint32(metadata.NodeCount),
		RecordSize:   uint16(metadata.RecordSize),
	}, nil
}

// IsDatabaseExpired 检查数据库是否过期
func (d *MaxmindDetector) IsDatabaseExpired() (bool, error) {
	if !d.initialized || d.db == nil {
		return false, fmt.Errorf("Maxmind database not initialized")
	}

	info, err := d.GetDatabaseInfo()
	if err != nil {
		return false, err
	}

	// 如果数据库超过90天，认为已过期
	expiryDate := info.BuildEpoch.AddDate(0, 0, 90)
	return time.Now().After(expiryDate), nil
}

// GetDatabaseAge 获取数据库年龄
func (d *MaxmindDetector) GetDatabaseAge() (time.Duration, error) {
	if !d.initialized || d.db == nil {
		return 0, fmt.Errorf("Maxmind database not initialized")
	}

	info, err := d.GetDatabaseInfo()
	if err != nil {
		return 0, err
	}

	return time.Since(info.BuildEpoch), nil
}

// ValidateIP 验证IP地址
func (d *MaxmindDetector) ValidateIP(ip string) error {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	// 检查IP是否在数据库范围内
	record, err := d.db.City(ipAddr)
	if err != nil {
		return fmt.Errorf("IP not found in database: %w", err)
	}

	if record.Country.IsoCode == "" {
		return fmt.Errorf("no country information found for IP: %s", ip)
	}

	return nil
}
