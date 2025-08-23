package detector

import (
	"testing"
	"time"
)

// 创建测试用的工厂实例
func createTestFactory() *detectorFactory {
	return &detectorFactory{
		creators: make(map[string]func(config DetectorConfig) (Detector, error)),
		info:     make(map[string]*DetectorInfo),
	}
}

func TestDetectorFactory_CreateDetector_Success(t *testing.T) {
	factory := createTestFactory()

	// 创建检测器信息
	info := &DetectorInfo{
		Type:        DetectorTypeGeolocation,
		Name:        "Mock Geolocation",
		Version:     "1.0.0",
		Description: "Mock geolocation detector for testing",
		Author:      "Test Author",
		License:     "MIT",
		Category:    "testing",
		Tags:        []string{"test", "mock"},
	}

	// 注册检测器
	err := factory.RegisterDetector("mock-geolocation", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("mock-geolocation", DetectorTypeGeolocation), nil
	}, info)

	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	config := DetectorConfig{
		Enabled:    true,
		Timeout:    30 * time.Second,
		RetryCount: 3,
	}

	detector, err := factory.CreateDetector("mock-geolocation", config)
	if err != nil {
		t.Errorf("Expected no error creating detector, got %v", err)
	}

	if detector == nil {
		t.Error("Expected detector, got nil")
	}

	if detector.GetName() != "mock-geolocation" {
		t.Errorf("Expected detector name 'mock-geolocation', got '%s'", detector.GetName())
	}
}

func TestDetectorFactory_CreateDetector_NotRegistered(t *testing.T) {
	factory := createTestFactory()

	config := DetectorConfig{
		Enabled: true,
	}

	detector, err := factory.CreateDetector("non-existent", config)
	if err == nil {
		t.Error("Expected error for non-existent detector, got none")
	}

	if detector != nil {
		t.Errorf("Expected nil detector, got %v", detector)
	}

	expectedErr := "detector non-existent not registered"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDetectorFactory_GetAvailableDetectors(t *testing.T) {
	factory := createTestFactory()

	// 初始状态应该没有检测器
	available := factory.GetAvailableDetectors()
	if len(available) != 0 {
		t.Errorf("Expected 0 available detectors, got %d", len(available))
	}

	// 注册几个检测器
	info1 := &DetectorInfo{
		Type:    DetectorTypeGeolocation,
		Name:    "Geolocation Test",
		Version: "1.0.0",
	}

	info2 := &DetectorInfo{
		Type:    DetectorTypeRisk,
		Name:    "Risk Test",
		Version: "1.0.0",
	}

	err := factory.RegisterDetector("geolocation-1", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("geolocation-1", DetectorTypeGeolocation), nil
	}, info1)
	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	err = factory.RegisterDetector("risk-1", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("risk-1", DetectorTypeRisk), nil
	}, info2)
	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	available = factory.GetAvailableDetectors()
	if len(available) != 2 {
		t.Errorf("Expected 2 available detectors, got %d", len(available))
	}

	// 检查包含预期的检测器类型
	foundGeo := false
	foundRisk := false
	for _, detectorType := range available {
		if detectorType == "geolocation-1" {
			foundGeo = true
		}
		if detectorType == "risk-1" {
			foundRisk = true
		}
	}

	if !foundGeo {
		t.Error("Expected to find 'geolocation-1' in available detectors")
	}
	if !foundRisk {
		t.Error("Expected to find 'risk-1' in available detectors")
	}
}

func TestDetectorFactory_RegisterDetector_Duplicate(t *testing.T) {
	factory := createTestFactory()

	info := &DetectorInfo{
		Type:    DetectorTypeGeolocation,
		Name:    "Duplicate Test",
		Version: "1.0.0",
	}

	// 首次注册
	err := factory.RegisterDetector("duplicate", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("duplicate-1", DetectorTypeGeolocation), nil
	}, info)
	if err != nil {
		t.Errorf("Expected no error on first registration, got %v", err)
	}

	// 重复注册应该失败
	err = factory.RegisterDetector("duplicate", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("duplicate-2", DetectorTypeRisk), nil
	}, info)
	if err == nil {
		t.Error("Expected error on duplicate registration, got none")
	}

	expectedErr := "detector duplicate already registered"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDetectorFactory_RegisterDetector_NilCreator(t *testing.T) {
	factory := createTestFactory()

	info := &DetectorInfo{
		Type:    DetectorTypeGeolocation,
		Name:    "Test",
		Version: "1.0.0",
	}

	err := factory.RegisterDetector("test", nil, info)
	if err == nil {
		t.Error("Expected error for nil creator, got none")
	}

	expectedErr := "creator function cannot be nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDetectorFactory_RegisterDetector_NilInfo(t *testing.T) {
	factory := createTestFactory()

	err := factory.RegisterDetector("test", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test", DetectorTypeGeolocation), nil
	}, nil)
	if err == nil {
		t.Error("Expected error for nil info, got none")
	}

	expectedErr := "detector info cannot be nil"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDetectorFactory_GetDetectorInfo_Success(t *testing.T) {
	factory := createTestFactory()

	expectedInfo := &DetectorInfo{
		Type:        DetectorTypeGeolocation,
		Name:        "Test Info Detector",
		Version:     "2.0.0",
		Description: "A test detector with info",
		Author:      "Test Author",
		License:     "MIT",
		Category:    "testing",
		Tags:        []string{"test", "mock"},
	}

	err := factory.RegisterDetector("test-info", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test-info", DetectorTypeGeolocation), nil
	}, expectedInfo)
	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	info, err := factory.GetDetectorInfo("test-info")
	if err != nil {
		t.Errorf("Expected no error getting detector info, got %v", err)
	}

	if info == nil {
		t.Error("Expected detector info, got nil")
	}

	if info.Name != "Test Info Detector" {
		t.Errorf("Expected name 'Test Info Detector', got '%s'", info.Name)
	}

	if info.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", info.Version)
	}

	if info.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", info.Author)
	}
}

func TestDetectorFactory_GetDetectorInfo_NotFound(t *testing.T) {
	factory := createTestFactory()

	info, err := factory.GetDetectorInfo("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent detector, got none")
	}

	if info != nil {
		t.Errorf("Expected nil info, got %v", info)
	}

	expectedErr := "detector non-existent not found"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDetectorFactory_ValidateConfig_Success(t *testing.T) {
	factory := createTestFactory()

	info := &DetectorInfo{
		Type:           DetectorTypeRisk,
		Name:           "Test Validator",
		Version:        "1.0.0",
		RequiredConfig: []string{"api_key"},
	}

	err := factory.RegisterDetector("test-validator", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test-validator", DetectorTypeRisk), nil
	}, info)
	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	config := DetectorConfig{
		Enabled: true,
		APIKey:  "test-api-key",
	}

	err = factory.ValidateConfig("test-validator", config)
	if err != nil {
		t.Errorf("Expected no error validating config, got %v", err)
	}
}

func TestDetectorFactory_ValidateConfig_MissingAPIKey(t *testing.T) {
	factory := createTestFactory()

	info := &DetectorInfo{
		Type:           DetectorTypeRisk,
		Name:           "Test Validator",
		Version:        "1.0.0",
		RequiredConfig: []string{"api_key"},
	}

	err := factory.RegisterDetector("test-validator", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test-validator", DetectorTypeRisk), nil
	}, info)
	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	config := DetectorConfig{
		Enabled: true,
		// APIKey missing
	}

	err = factory.ValidateConfig("test-validator", config)
	if err == nil {
		t.Error("Expected error for missing API key, got none")
	}

	expectedErr := "required configuration 'api_key' is missing"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestDetectorFactory_CreateDetector_MissingRequiredConfig(t *testing.T) {
	factory := createTestFactory()

	info := &DetectorInfo{
		Type:           DetectorTypeGeolocation,
		Name:           "Test Required Config",
		Version:        "1.0.0",
		RequiredConfig: []string{"endpoint"},
	}

	err := factory.RegisterDetector("test-required", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test-required", DetectorTypeGeolocation), nil
	}, info)
	if err != nil {
		t.Errorf("Expected no error registering detector, got %v", err)
	}

	config := DetectorConfig{
		Enabled: true,
		// Endpoint missing
	}

	detector, err := factory.CreateDetector("test-required", config)
	if err == nil {
		t.Error("Expected error for missing required config, got none")
	}

	if detector != nil {
		t.Errorf("Expected nil detector, got %v", detector)
	}

	expectedErr := "required configuration 'endpoint' is missing for detector test-required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestBuiltinDetectorInfo(t *testing.T) {
	// 测试获取内置检测器信息
	info, err := GetBuiltinDetectorInfo("maxmind")
	if err != nil {
		t.Errorf("Expected no error getting builtin detector info, got %v", err)
	}

	if info == nil {
		t.Error("Expected detector info, got nil")
	}

	if info.Name != "MaxMind GeoIP" {
		t.Errorf("Expected name 'MaxMind GeoIP', got '%s'", info.Name)
	}

	if info.Type != DetectorTypeGeolocation {
		t.Errorf("Expected type '%s', got '%s'", DetectorTypeGeolocation, info.Type)
	}

	// 测试不存在的内置检测器
	info, err = GetBuiltinDetectorInfo("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent builtin detector, got none")
	}

	if info != nil {
		t.Errorf("Expected nil info, got %v", info)
	}
}

func TestGetAllBuiltinDetectorInfo(t *testing.T) {
	allInfo := GetAllBuiltinDetectorInfo()

	if len(allInfo) == 0 {
		t.Error("Expected at least one builtin detector info")
	}

	// 检查是否包含预期的内置检测器
	expectedDetectors := []string{"maxmind", "ipinfo", "scamalytics", "streaming", "ai"}
	for _, expected := range expectedDetectors {
		if _, exists := allInfo[expected]; !exists {
			t.Errorf("Expected builtin detector '%s' not found", expected)
		}
	}

	// 验证信息完整性
	for detectorType, info := range allInfo {
		if info.Name == "" {
			t.Errorf("Detector '%s' has empty name", detectorType)
		}
		if info.Version == "" {
			t.Errorf("Detector '%s' has empty version", detectorType)
		}
		if info.Type == "" {
			t.Errorf("Detector '%s' has empty type", detectorType)
		}
	}
}

func TestGlobalDetectorFactory(t *testing.T) {
	// 测试全局工厂功能
	info := &DetectorInfo{
		Type:    DetectorTypeService,
		Name:    "Global Test",
		Version: "1.0.0",
	}

	err := RegisterGlobalDetector("global-test", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("global-test", DetectorTypeService), nil
	}, info)
	if err != nil {
		t.Errorf("Expected no error registering global detector, got %v", err)
	}

	// 验证在可用检测器列表中
	available := GetAvailableGlobalDetectors()
	found := false
	for _, detectorType := range available {
		if detectorType == "global-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'global-test' in available global detectors")
	}

	// 测试创建全局检测器
	config := DetectorConfig{Enabled: true}
	detector, err := CreateGlobalDetector("global-test", config)
	if err != nil {
		t.Errorf("Expected no error creating global detector, got %v", err)
	}

	if detector == nil {
		t.Error("Expected detector, got nil")
	}

	// 测试获取全局检测器信息
	globalInfo, err := GetGlobalDetectorInfo("global-test")
	if err != nil {
		t.Errorf("Expected no error getting global detector info, got %v", err)
	}

	if globalInfo.Name != "Global Test" {
		t.Errorf("Expected name 'Global Test', got '%s'", globalInfo.Name)
	}
}

func TestGetAllDetectorInfo(t *testing.T) {
	factory := createTestFactory()

	// 注册几个检测器
	info1 := &DetectorInfo{Type: DetectorTypeGeolocation, Name: "Test 1", Version: "1.0.0"}
	info2 := &DetectorInfo{Type: DetectorTypeRisk, Name: "Test 2", Version: "1.0.0"}

	_ = factory.RegisterDetector("test-1", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test-1", DetectorTypeGeolocation), nil
	}, info1)

	_ = factory.RegisterDetector("test-2", func(config DetectorConfig) (Detector, error) {
		return NewMockDetector("test-2", DetectorTypeRisk), nil
	}, info2)

	allInfo := factory.GetAllDetectorInfo()
	if len(allInfo) != 2 {
		t.Errorf("Expected 2 detector infos, got %d", len(allInfo))
	}

	if allInfo["test-1"] == nil {
		t.Error("Expected test-1 info to exist")
	}

	if allInfo["test-2"] == nil {
		t.Error("Expected test-2 info to exist")
	}
}
