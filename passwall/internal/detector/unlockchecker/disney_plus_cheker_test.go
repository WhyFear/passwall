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
		Config: "your config here",
	})

	resp := checker.Check(ipProxy)
	assert.NotNil(t, resp)
}
