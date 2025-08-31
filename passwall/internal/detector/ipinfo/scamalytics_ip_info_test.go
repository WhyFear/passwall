package ipinfo

import (
	"net/http"
	"passwall/internal/model"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestNewScamalyticsRiskDetector(t *testing.T) {
	scamalyticsDetector := NewScamalyticsRiskDetector()
	assert.NotNil(t, scamalyticsDetector)

	// 验证返回的是正确的类型
	_, ok := scamalyticsDetector.(*ScamalyticsRiskDetector)
	assert.True(t, ok)
}

func TestScamalyticsRiskDetector_Detect_Success(t *testing.T) {
	httpmock.Activate(t)
	httpmock.RegisterResponder("GET", "https://scamalytics.com/ip/1.1.1.1",
		httpmock.NewStringResponder(200, `		<html>
			<body>
				<div>Fraud Score: 75</div>
				<div>Some other content</div>
			</body>
		</html>`))
	scamalyticsDetector := NewScamalyticsRiskDetector()
	detect, err := scamalyticsDetector.Detect(&model.IPProxy{
		IP:          "1.1.1.1",
		ProxyClient: new(http.Client)})
	assert.NoError(t, err)
	assert.Equal(t, 75, detect.Risk.Score)
}

func TestScamalyticsRiskDetector_Detect_NoScoreFound(t *testing.T) {
	httpmock.Activate(t)
	httpmock.RegisterResponder("GET", "https://scamalytics.com/ip/1.1.1.1",
		httpmock.NewStringResponder(200, `		<html>
			<body>
				<div>No fraud score here</div>
			</body>
		</html>`))
	scamalyticsDetector := NewScamalyticsRiskDetector()
	detect, err := scamalyticsDetector.Detect(&model.IPProxy{
		IP:          "1.1.1.1",
		ProxyClient: new(http.Client)})
	assert.NoError(t, err)
	// 当没有分数信息时，应该返回 -1
	assert.Equal(t, -1, detect.Risk.Score)
}

func TestScamalyticsRiskDetector_Detect_InvalidScore(t *testing.T) {
	httpmock.Activate(t)
	httpmock.RegisterResponder("GET", "https://scamalytics.com/ip/1.1.1.1",
		httpmock.NewStringResponder(200, `		<html>
			<body>
				<div>Fraud Score: invalid</div>
			</body>
		</html>`))
	scamalyticsDetector := NewScamalyticsRiskDetector()
	detect, err := scamalyticsDetector.Detect(&model.IPProxy{
		IP:          "1.1.1.1",
		ProxyClient: new(http.Client)})
	assert.NoError(t, err)
	// 当分数转换失败时，应该返回 -1
	assert.Equal(t, -1, detect.Risk.Score)
}

func TestScamalyticsRiskDetector_Detect_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		response string
		expected int
	}{
		{
			name:     "高分情况",
			response: `<div>Fraud Score: 100</div>`,
			expected: 100,
		},
		{
			name:     "低分情况",
			response: `<div>Fraud Score: 0</div>`,
			expected: 0,
		},
		{
			name:     "多位数字",
			response: `<div>Fraud Score: 123</div>`,
			expected: 123,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate(t)
			httpmock.RegisterResponder("GET", "https://scamalytics.com/ip/1.1.1.1",
				httpmock.NewStringResponder(200, tc.response))
			scamalyticsDetector := NewScamalyticsRiskDetector()
			detect, err := scamalyticsDetector.Detect(&model.IPProxy{
				IP:          "1.1.1.1",
				ProxyClient: new(http.Client)})
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, detect.Risk.Score)
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
			name:         "有效的国家代码",
			response:     `<html><body><div>Fraud Score: 100</div><table><tr><th>Country Code</th><td>US</td></tr></table></body></html>`,
			expectedCode: "US",
		},
		{
			name:         "没有国家代码表格",
			response:     `<html><body><div>Fraud Score: 100</div><div>No country code here</div></body></html>`,
			expectedCode: "",
		},
		{
			name:         "国家代码格式正确但内容为空",
			response:     `<html><body><div>Fraud Score: 100</div><table><tr><th>Country Code</th><td></td></tr></table></body></html>`,
			expectedCode: "",
		},
		{
			name:         "小写国家代码,取不到",
			response:     `<html><body><div>Fraud Score: 100</div><table><tr><th>Country Code</th><td>us</td></tr></table></body></html>`,
			expectedCode: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate(t)
			httpmock.RegisterResponder("GET", "https://scamalytics.com/ip/1.1.1.1",
				httpmock.NewStringResponder(200, tc.response))
			scamalyticsDetector := NewScamalyticsRiskDetector()
			detect, err := scamalyticsDetector.Detect(&model.IPProxy{
				IP:          "1.1.1.1",
				ProxyClient: new(http.Client)})
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCode, detect.Geo.CountryCode)
		})
	}
}
