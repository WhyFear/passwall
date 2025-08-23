package detector

import (
	"context"
	"testing"
	"time"
)

// MockDetector 用于测试的模拟检测器
type MockDetector struct {
	name         string
	version      string
	detectorType DetectorType
	status       DetectorStatus
	config       DetectorConfig
	enabled      bool
	shouldError  bool
	result       *DetectorResult
	delay        time.Duration
}

func NewMockDetector(name string, detectorType DetectorType) *MockDetector {
	return &MockDetector{
		name:         name,
		version:      "1.0.0",
		detectorType: detectorType,
		status:       DetectorStatusAvailable,
		enabled:      true,
		config: DetectorConfig{
			Enabled:    true,
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: time.Second,
		},
	}
}

func (m *MockDetector) GetType() DetectorType {
	return m.detectorType
}

func (m *MockDetector) GetName() string {
	return m.name
}

func (m *MockDetector) GetVersion() string {
	return m.version
}

func (m *MockDetector) GetStatus() DetectorStatus {
	return m.status
}

func (m *MockDetector) GetConfig() DetectorConfig {
	return m.config
}

func (m *MockDetector) SetConfig(config DetectorConfig) error {
	m.config = config
	return nil
}

func (m *MockDetector) TestConnection(ctx context.Context) error {
	if m.shouldError {
		return &DetectionError{
			Detector: m.name,
			IP:       "",
			Message:  "connection test failed",
		}
	}
	return nil
}

func (m *MockDetector) Detect(ctx context.Context, ip string) (*DetectorResult, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldError {
		return nil, &DetectionError{
			Detector: m.name,
			IP:       ip,
			Message:  "mock detection error",
		}
	}

	if m.result != nil {
		return m.result, nil
	}

	return &DetectorResult{
		Success:   true,
		Data:      map[string]interface{}{"test": "data"},
		Timestamp: time.Now(),
		Provider:  m.name,
	}, nil
}

func (m *MockDetector) GetProvider() string {
	return m.name
}

func (m *MockDetector) IsEnabled() bool {
	return m.enabled
}

func (m *MockDetector) SetEnabled(enabled bool) error {
	m.enabled = enabled
	return nil
}

// 测试辅助方法
func (m *MockDetector) SetShouldError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockDetector) SetResult(result *DetectorResult) {
	m.result = result
}

func (m *MockDetector) SetDelay(delay time.Duration) {
	m.delay = delay
}

func (m *MockDetector) SetStatus(status DetectorStatus) {
	m.status = status
}

// 测试用例

func TestDetectionError(t *testing.T) {
	// 测试基本错误信息
	err := &DetectionError{
		Detector: "test-detector",
		IP:       "127.0.0.1",
		Message:  "test error",
	}

	expected := "detector 'test-detector' failed to detect IP '127.0.0.1': test error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}

	// 测试包装错误
	wrappedErr := &DetectionError{
		Detector: "test-detector",
		IP:       "127.0.0.1",
		Message:  "test error",
		Err:      context.DeadlineExceeded,
	}

	if wrappedErr.Unwrap() != context.DeadlineExceeded {
		t.Errorf("Expected wrapped error to be context.DeadlineExceeded, got %v", wrappedErr.Unwrap())
	}
}

func TestMockDetector_BasicInfo(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeGeolocation)

	if detector.GetName() != "test-detector" {
		t.Errorf("Expected detector name 'test-detector', got '%s'", detector.GetName())
	}

	if detector.GetType() != DetectorTypeGeolocation {
		t.Errorf("Expected detector type '%s', got '%s'", DetectorTypeGeolocation, detector.GetType())
	}

	if detector.GetVersion() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", detector.GetVersion())
	}

	if detector.GetProvider() != "test-detector" {
		t.Errorf("Expected provider 'test-detector', got '%s'", detector.GetProvider())
	}

	if !detector.IsEnabled() {
		t.Error("Expected detector to be enabled")
	}
}

func TestMockDetector_Config(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeRisk)

	config := detector.GetConfig()
	if !config.Enabled {
		t.Error("Expected config to be enabled")
	}

	newConfig := DetectorConfig{
		Enabled:    false,
		Timeout:    60 * time.Second,
		RetryCount: 5,
	}

	err := detector.SetConfig(newConfig)
	if err != nil {
		t.Errorf("Expected no error setting config, got %v", err)
	}

	updatedConfig := detector.GetConfig()
	if updatedConfig.Enabled {
		t.Error("Expected config to be disabled")
	}

	if updatedConfig.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", updatedConfig.Timeout)
	}
}

func TestMockDetector_EnabledState(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeService)

	if !detector.IsEnabled() {
		t.Error("Expected detector to be enabled initially")
	}

	err := detector.SetEnabled(false)
	if err != nil {
		t.Errorf("Expected no error disabling detector, got %v", err)
	}

	if detector.IsEnabled() {
		t.Error("Expected detector to be disabled")
	}

	err = detector.SetEnabled(true)
	if err != nil {
		t.Errorf("Expected no error enabling detector, got %v", err)
	}

	if !detector.IsEnabled() {
		t.Error("Expected detector to be enabled")
	}
}

func TestMockDetector_TestConnection_Success(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeGeolocation)

	ctx := context.Background()
	err := detector.TestConnection(ctx)
	if err != nil {
		t.Errorf("Expected no error testing connection, got %v", err)
	}
}

func TestMockDetector_TestConnection_Error(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeGeolocation)
	detector.SetShouldError(true)

	ctx := context.Background()
	err := detector.TestConnection(ctx)
	if err == nil {
		t.Error("Expected error testing connection, got none")
	}

	detectionErr, ok := err.(*DetectionError)
	if !ok {
		t.Errorf("Expected DetectionError, got %T", err)
	}

	if detectionErr.Detector != "test-detector" {
		t.Errorf("Expected detector 'test-detector', got '%s'", detectionErr.Detector)
	}
}

func TestMockDetector_Detect_Success(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeGeolocation)

	ctx := context.Background()
	result, err := detector.Detect(ctx, "127.0.0.1")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	if result.Provider != "test-detector" {
		t.Errorf("Expected provider 'test-detector', got '%s'", result.Provider)
	}
}

func TestMockDetector_Detect_CustomResult(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeRisk)

	customResult := &DetectorResult{
		Success:   true,
		Data:      map[string]interface{}{"custom": "data"},
		Timestamp: time.Now(),
		Provider:  "custom-provider",
	}

	detector.SetResult(customResult)

	ctx := context.Background()
	result, err := detector.Detect(ctx, "127.0.0.1")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	if result.Data["custom"] != "data" {
		t.Errorf("Expected custom data 'data', got %v", result.Data["custom"])
	}
}

func TestMockDetector_Detect_Error(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeService)
	detector.SetShouldError(true)

	ctx := context.Background()
	result, err := detector.Detect(ctx, "127.0.0.1")

	if err == nil {
		t.Error("Expected error, got none")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	detectionErr, ok := err.(*DetectionError)
	if !ok {
		t.Errorf("Expected DetectionError, got %T", err)
	}

	if detectionErr.IP != "127.0.0.1" {
		t.Errorf("Expected IP '127.0.0.1', got '%s'", detectionErr.IP)
	}
}

func TestMockDetector_Detect_Timeout(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeGeolocation)
	detector.SetDelay(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := detector.Detect(ctx, "127.0.0.1")

	if err == nil {
		t.Error("Expected timeout error, got none")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestMockDetector_Status(t *testing.T) {
	detector := NewMockDetector("test-detector", DetectorTypeRisk)

	if detector.GetStatus() != DetectorStatusAvailable {
		t.Errorf("Expected status '%s', got '%s'", DetectorStatusAvailable, detector.GetStatus())
	}

	detector.SetStatus(DetectorStatusError)
	if detector.GetStatus() != DetectorStatusError {
		t.Errorf("Expected status '%s', got '%s'", DetectorStatusError, detector.GetStatus())
	}
}
