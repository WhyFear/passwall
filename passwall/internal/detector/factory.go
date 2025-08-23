package detector

import (
	"fmt"
	"sync"
)

// detectorFactory 检测器工厂实现
type detectorFactory struct {
	creators map[string]func(config DetectorConfig) (Detector, error)
	info     map[string]*DetectorInfo
	mu       sync.RWMutex
}

// NewDetectorFactory 创建新的检测器工厂
func NewDetectorFactory() DetectorFactory {
	return &detectorFactory{
		creators: make(map[string]func(config DetectorConfig) (Detector, error)),
		info:     make(map[string]*DetectorInfo),
	}
}

// RegisterDetector 注册检测器创建器
func (df *detectorFactory) RegisterDetector(
	detectorType string,
	creator func(config DetectorConfig) (Detector, error),
	info *DetectorInfo,
) error {
	if creator == nil {
		return fmt.Errorf("creator function cannot be nil")
	}
	if info == nil {
		return fmt.Errorf("detector info cannot be nil")
	}

	df.mu.Lock()
	defer df.mu.Unlock()

	if _, exists := df.creators[detectorType]; exists {
		return fmt.Errorf("detector %s already registered", detectorType)
	}

	df.creators[detectorType] = creator
	df.info[detectorType] = info

	return nil
}

// CreateDetector 创建检测器
func (df *detectorFactory) CreateDetector(detectorType string, config DetectorConfig) (Detector, error) {
	df.mu.RLock()
	creator, exists := df.creators[detectorType]
	df.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("detector %s not registered", detectorType)
	}

	// 验证必需配置
	if info, ok := df.info[detectorType]; ok {
		for _, requiredConfig := range info.RequiredConfig {
			if requiredConfig == "api_key" && config.APIKey == "" {
				return nil, fmt.Errorf("required configuration 'api_key' is missing for detector %s", detectorType)
			}
			if requiredConfig == "endpoint" && config.Endpoint == "" {
				return nil, fmt.Errorf("required configuration 'endpoint' is missing for detector %s", detectorType)
			}
		}
	}

	return creator(config)
}

// GetAvailableDetectors 获取可用的检测器列表
func (df *detectorFactory) GetAvailableDetectors() []string {
	df.mu.RLock()
	defer df.mu.RUnlock()

	detectors := make([]string, 0, len(df.creators))
	for detectorType := range df.creators {
		detectors = append(detectors, detectorType)
	}

	return detectors
}

// GetDetectorInfo 获取检测器信息
func (df *detectorFactory) GetDetectorInfo(detectorType string) (*DetectorInfo, error) {
	df.mu.RLock()
	defer df.mu.RUnlock()

	info, exists := df.info[detectorType]
	if !exists {
		return nil, fmt.Errorf("detector %s not found", detectorType)
	}

	return info, nil
}

// GetAllDetectorInfo 获取所有检测器信息
func (df *detectorFactory) GetAllDetectorInfo() map[string]*DetectorInfo {
	df.mu.RLock()
	defer df.mu.RUnlock()

	infoCopy := make(map[string]*DetectorInfo)
	for detectorType, info := range df.info {
		infoCopy[detectorType] = info
	}

	return infoCopy
}

// ValidateConfig 验证配置
func (df *detectorFactory) ValidateConfig(detectorType string, config DetectorConfig) error {
	info, err := df.GetDetectorInfo(detectorType)
	if err != nil {
		return err
	}

	// 验证必需配置
	for _, requiredConfig := range info.RequiredConfig {
		switch requiredConfig {
		case "api_key":
			if config.APIKey == "" {
				return fmt.Errorf("required configuration 'api_key' is missing")
			}
		case "endpoint":
			if config.Endpoint == "" {
				return fmt.Errorf("required configuration 'endpoint' is missing")
			}
		}
	}

	// 验证配置模式（如果有）
	if info.ConfigSchema != nil {
		// 这里可以添加更复杂的配置验证逻辑
		// 目前只做基本验证
	}

	return nil
}

// GlobalDetectorFactory 全局检测器工厂实例
var GlobalDetectorFactory DetectorFactory = NewDetectorFactory()

// RegisterGlobalDetector 注册全局检测器
func RegisterGlobalDetector(
	detectorType string,
	creator func(config DetectorConfig) (Detector, error),
	info *DetectorInfo,
) error {
	if factory, ok := GlobalDetectorFactory.(*detectorFactory); ok {
		return factory.RegisterDetector(detectorType, creator, info)
	}
	return fmt.Errorf("global detector factory is not properly initialized")
}

// CreateGlobalDetector 创建全局检测器
func CreateGlobalDetector(detectorType string, config DetectorConfig) (Detector, error) {
	return GlobalDetectorFactory.CreateDetector(detectorType, config)
}

// GetAvailableGlobalDetectors 获取可用的全局检测器
func GetAvailableGlobalDetectors() []string {
	return GlobalDetectorFactory.GetAvailableDetectors()
}

// GetGlobalDetectorInfo 获取全局检测器信息
func GetGlobalDetectorInfo(detectorType string) (*DetectorInfo, error) {
	return GlobalDetectorFactory.GetDetectorInfo(detectorType)
}

// GetAllGlobalDetectorInfo 获取所有全局检测器信息
func GetAllGlobalDetectorInfo() map[string]*DetectorInfo {
	if factory, ok := GlobalDetectorFactory.(*detectorFactory); ok {
		return factory.GetAllDetectorInfo()
	}
	return make(map[string]*DetectorInfo)
}

// builtinDetectorInfos 内置检测器信息
var builtinDetectorInfos = map[string]*DetectorInfo{
	"maxmind": {
		Type:           DetectorTypeGeolocation,
		Name:           "MaxMind GeoIP",
		Version:        "1.0.0",
		Description:    "MaxMind GeoIP database for geolocation detection",
		Author:         "PassWall Team",
		Website:        "https://www.maxmind.com",
		License:        "Commercial",
		Category:       "geolocation",
		Tags:           []string{"geolocation", "ip", "database"},
		RequiredConfig: []string{"database_path"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"database_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to MaxMind database file",
				},
				"cache_enabled": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable caching",
					"default":     true,
				},
			},
		},
	},
	"ipinfo": {
		Type:           DetectorTypeGeolocation,
		Name:           "IPinfo",
		Version:        "1.0.0",
		Description:    "IPinfo.io IP geolocation and intelligence service",
		Author:         "PassWall Team",
		Website:        "https://ipinfo.io",
		License:        "Commercial",
		Category:       "geolocation",
		Tags:           []string{"geolocation", "ip", "api"},
		RequiredConfig: []string{"api_key"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"api_key": map[string]interface{}{
					"type":        "string",
					"description": "IPinfo API key",
				},
				"endpoint": map[string]interface{}{
					"type":        "string",
					"description": "IPinfo API endpoint",
					"default":     "https://ipinfo.io",
				},
			},
		},
	},
	"scamalytics": {
		Type:           DetectorTypeRisk,
		Name:           "Scamalytics",
		Version:        "1.0.0",
		Description:    "Scamalytics IP fraud detection service",
		Author:         "PassWall Team",
		Website:        "https://scamalytics.com",
		License:        "Commercial",
		Category:       "risk",
		Tags:           []string{"risk", "fraud", "api"},
		RequiredConfig: []string{"api_key"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"api_key": map[string]interface{}{
					"type":        "string",
					"description": "Scamalytics API key",
				},
				"endpoint": map[string]interface{}{
					"type":        "string",
					"description": "Scamalytics API endpoint",
					"default":     "https://scamalytics.com",
				},
			},
		},
	},
	"ipqs": {
		Type:           DetectorTypeRisk,
		Name:           "IPQualityScore",
		Version:        "1.0.0",
		Description:    "IPQualityScore IP fraud detection and scoring service",
		Author:         "PassWall Team",
		Website:        "https://www.ipqualityscore.com",
		License:        "Commercial",
		Category:       "risk",
		Tags:           []string{"risk", "fraud", "scoring", "api"},
		RequiredConfig: []string{"api_key"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"api_key": map[string]interface{}{
					"type":        "string",
					"description": "IPQualityScore API key",
				},
				"endpoint": map[string]interface{}{
					"type":        "string",
					"description": "IPQualityScore API endpoint",
					"default":     "https://www.ipqualityscore.com",
				},
				"strictness": map[string]interface{}{
					"type":        "integer",
					"description": "Strictness level (0-3)",
					"default":     0,
				},
			},
		},
	},
	"streaming": {
		Type:        DetectorTypeService,
		Name:        "Streaming Services",
		Version:     "1.0.0",
		Description: "Streaming services unlock detection",
		Author:      "PassWall Team",
		License:     "MIT",
		Category:    "service",
		Tags:        []string{"streaming", "unlock", "netflix", "disney"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Request timeout in seconds",
					"default":     30,
				},
				"services": map[string]interface{}{
					"type":        "array",
					"description": "List of services to check",
					"default":     []string{"netflix", "disney", "youtube", "amazon"},
				},
			},
		},
	},
	"ai": {
		Type:        DetectorTypeService,
		Name:        "AI Services",
		Version:     "1.0.0",
		Description: "AI services unlock detection",
		Author:      "PassWall Team",
		License:     "MIT",
		Category:    "service",
		Tags:        []string{"ai", "unlock", "chatgpt", "claude"},
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Request timeout in seconds",
					"default":     30,
				},
				"services": map[string]interface{}{
					"type":        "array",
					"description": "List of AI services to check",
					"default":     []string{"chatgpt", "claude", "gemini", "copilot"},
				},
			},
		},
	},
}

// GetBuiltinDetectorInfo 获取内置检测器信息
func GetBuiltinDetectorInfo(detectorType string) (*DetectorInfo, error) {
	info, exists := builtinDetectorInfos[detectorType]
	if !exists {
		return nil, fmt.Errorf("builtin detector %s not found", detectorType)
	}
	return info, nil
}

// GetAllBuiltinDetectorInfo 获取所有内置检测器信息
func GetAllBuiltinDetectorInfo() map[string]*DetectorInfo {
	infoCopy := make(map[string]*DetectorInfo)
	for detectorType, info := range builtinDetectorInfos {
		infoCopy[detectorType] = info
	}
	return infoCopy
}
