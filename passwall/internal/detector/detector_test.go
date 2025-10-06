package detector

import (
	"passwall/config"
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDetectorManager(t *testing.T) {
	// 创建一个空的配置用于测试
	cfg := config.Config{}
	manager := NewDetectorManager(cfg)
	assert.NotNil(t, manager)
	resp, err := manager.DetectAll(model.NewIPProxy(&model.Proxy{
		Config: "you config here",
	}), true, true)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
