package ipinfo

import (
	"net/http"
	"passwall/config"
	"passwall/internal/model"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

var scamalyticsConfig = &config.Scamalytics{
	Host:   "https://api11.scamalytics.com/v3/",
	APIKey: "test-api-key",
	User:   "test-user",
}
var apiUrl = scamalyticsConfig.Host + "/" + scamalyticsConfig.User + "?key=" + scamalyticsConfig.APIKey + "&ip="

func TestNewScamalyticsRiskDetector(t *testing.T) {
	scamalyticsDetector := NewScamalyticsRiskDetector(*scamalyticsConfig)
	assert.NotNil(t, scamalyticsDetector)

	// 验证返回的是正确的类型
	_, ok := scamalyticsDetector.(*ScamalyticsRiskDetector)
	assert.True(t, ok)
}

func TestScamalyticsRiskDetector_Detect_Success(t *testing.T) {
	httpmock.Activate(t)
	httpmock.RegisterResponder("GET", apiUrl+"1.1.1.1",
		httpmock.NewStringResponder(200, `{
  "scamalytics": {
    "scamalytics_score": 0,
    "scamalytics_risk": "low"
  }
}`))
	scamalyticsDetector := NewScamalyticsRiskDetector(*scamalyticsConfig)
	detect, err := scamalyticsDetector.Detect(&model.IPProxy{
		IP:          "1.1.1.1",
		ProxyClient: new(http.Client)})
	assert.NoError(t, err)
	assert.Equal(t, IPRiskTypeLow, detect.Risk.IPRiskType)
}

func TestScamalyticsRiskDetector_Detect_NoScoreFound(t *testing.T) {
	httpmock.Activate(t)
	httpmock.RegisterResponder("GET", apiUrl+"1.1.1.1",
		httpmock.NewStringResponder(200, `{
  "scamalytics": {
  }
}`))
	scamalyticsDetector := NewScamalyticsRiskDetector(*scamalyticsConfig)
	detect, err := scamalyticsDetector.Detect(&model.IPProxy{
		IP:          "1.1.1.1",
		ProxyClient: new(http.Client)})
	assert.NoError(t, err)
	assert.Equal(t, IPRiskTypeDetectFailed, detect.Risk.IPRiskType)
}

func TestScamalyticsRiskDetector_Detect_InvalidScore(t *testing.T) {
	httpmock.Activate(t)
	httpmock.RegisterResponder("GET", apiUrl+"1.1.1.1",
		httpmock.NewStringResponder(200, `{
  "scamalytics": {
    "scamalytics_score": 0,
    "scamalytics_risk": "lowb"
  }
}`))
	scamalyticsDetector := NewScamalyticsRiskDetector(*scamalyticsConfig)
	detect, err := scamalyticsDetector.Detect(&model.IPProxy{
		IP:          "1.1.1.1",
		ProxyClient: new(http.Client)})
	assert.NoError(t, err)
	assert.Equal(t, IPRiskTypeDetectFailed, detect.Risk.IPRiskType)
}

func TestScamalyticsRiskDetector_Detect_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		response string
		expected IPRiskType
	}{
		{
			name: "高分情况",
			response: `{
  "scamalytics": {
    "scamalytics_risk": "low"
  }
}`,
			expected: IPRiskTypeLow,
		},
		{
			name: "低分情况",
			response: `{
  "scamalytics": {
    "scamalytics_risk": "high"
  }
}`,
			expected: IPRiskTypeHigh,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate(t)
			httpmock.RegisterResponder("GET", apiUrl+"1.1.1.1",
				httpmock.NewStringResponder(200, tc.response))
			scamalyticsDetector := NewScamalyticsRiskDetector(*scamalyticsConfig)
			detect, err := scamalyticsDetector.Detect(&model.IPProxy{
				IP:          "1.1.1.1",
				ProxyClient: new(http.Client)})
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, detect.Risk.IPRiskType)
		})
	}
}

func TestScamalyticsRiskDetector_Detect_CountryCode(t *testing.T) {
	testCases := []struct {
		name         string
		response     string
		expectedCode string
	}{
		{
			name: "有效的国家代码",
			response: `{
  "external_datasources": {
    "ip2proxy_lite": {
      "ip_country_code": "US"
    }
  }
}`,
			expectedCode: "US",
		},
		{
			name: "没有国家代码表格",
			response: `{
  "external_datasources": {
    "ip2proxy_lite": {
    }
  }
}`,
			expectedCode: "",
		},
		{
			name: "国家代码格式正确但内容为空",
			response: `{
  "external_datasources": {
    "ip2proxy_lite": {
      "ip_country_code": ""
    }
  }
}`,
			expectedCode: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate(t)
			httpmock.RegisterResponder("GET", apiUrl+"1.1.1.1",
				httpmock.NewStringResponder(200, tc.response))
			scamalyticsDetector := NewScamalyticsRiskDetector(*scamalyticsConfig)
			detect, err := scamalyticsDetector.Detect(&model.IPProxy{
				IP:          "1.1.1.1",
				ProxyClient: new(http.Client)})
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCode, detect.Geo.CountryCode)
		})
	}
}
