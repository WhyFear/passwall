package unlockchecker

import (
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDisneyPlusChecker(t *testing.T) {
	checker := NewDisneyPlusChecker()

	if checker == nil {
		t.Fatal("Expected a valid DisneyPlusChecker, got nil")
	}
	ipProxy := model.NewIPProxy("1.1.1.1", &model.Proxy{
		Config: "{\"alterId\":1,\"cipher\":\"auto\",\"name\":\"🇭🇰 香港A10 | IEPL\",\"network\":\"ws\",\"port\":13486,\"server\":\"up0m7-g05.hk10-vm5.entry.v50708.dev\",\"skip-cert-verify\":false,\"tls\":false,\"type\":\"vmess\",\"udp\":true,\"uuid\":\"8bb86245-035c-3d9c-b139-b695a8b228d2\",\"ws-opts\":{\"headers\":{\"Host\":\"bgp-01-10.entry-0.chinasnow.net\"},\"path\":\"/\"}}",
	})

	resp := checker.Check(ipProxy)
	assert.NotNil(t, resp)
}
