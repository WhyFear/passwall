package detector

import (
	"passwall/config"
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDetectorManager(t *testing.T) {
	cfg := config.Config{}
	manager := NewDetectorManager(cfg)
	assert.NotNil(t, manager)

	resp, err := manager.DetectAll(&model.IPProxy{
		IPV4: "203.0.113.10",
	}, false, false)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "203.0.113.10", resp.BaseInfo.IPV4)
}

func TestDetectorManagerDetectAllRejectsNilProxy(t *testing.T) {
	manager := NewDetectorManager(config.Config{})
	resp, err := manager.DetectAll(nil, false, false)

	assert.Error(t, err)
	assert.Nil(t, resp)
}
