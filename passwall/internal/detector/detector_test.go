package detector

import (
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDetectorManager(t *testing.T) {
	manager := NewDetectorManager()
	assert.NotNil(t, manager)
	resp, err := manager.DetectAll(model.NewIPProxy("", &model.Proxy{
		Config: "{\"alpn\":[\"h2\",\"http/1.1\"],\"client-fingerprint\":\"chrome\",\"http-opts\":{\"headers\":{},\"path\":[\"/\"]},\"name\":\"linuxdo-linuxdo日本直连\",\"network\":\"tcp\",\"port\":\"45633\",\"server\":\"jp.functen.cn\",\"servername\":\"jp.functen.cn\",\"tls\":true,\"type\":\"vless\",\"udp\":true,\"uuid\":\"bd225cc6-7af5-4288-aff4-65e0ce121ce9\",\"xudp\":true}",
	}), true, true)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
