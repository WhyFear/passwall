package detector

import (
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDetectorManager(t *testing.T) {
	manager := NewDetectorManager()
	assert.NotNil(t, manager)
	resp, err := manager.DetectAll(model.NewIPProxy(&model.Proxy{
		Config: "you config here",
	}), true, true)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
