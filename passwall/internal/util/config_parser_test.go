package util

import (
	"passwall/internal/model"
	"testing"
)

func TestGetPasswordFromConfig(t *testing.T) {
	testCases := []struct {
		name      string
		proxyType model.ProxyType
		config    map[string]any
		expected  string
	}{
		{
			name:      "ss with password",
			proxyType: model.ProxyTypeSS,
			config: map[string]any{
				"password": "ss-password",
			},
			expected: "ss-password",
		},
		{
			name:      "vmess with uuid",
			proxyType: model.ProxyTypeVMess,
			config: map[string]any{
				"uuid": "vmess-uuid",
			},
			expected: "vmess-uuid",
		},
		{
			name:      "trojan with password",
			proxyType: model.ProxyTypeTrojan,
			config: map[string]any{
				"password": "trojan-password",
			},
			expected: "trojan-password",
		},
		{
			name:      "no password field",
			proxyType: model.ProxyTypeSS,
			config:    map[string]any{},
			expected:  "",
		},
		{
			name:      "unsupported type",
			proxyType: "unknown",
			config: map[string]any{
				"password": "some-password",
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := GetPasswordFromConfig(tc.proxyType, tc.config)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}
