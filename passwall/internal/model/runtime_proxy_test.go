package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeProxyNameRoundTrip(t *testing.T) {
	proxy := Proxy{ID: 42, Name: "same-name"}

	assert.Equal(t, "[pw:42]-same-name", proxy.RuntimeName(0))
	assert.Equal(t, "[pw:42]-[3]-same-name", proxy.RuntimeName(3))

	for _, name := range []string{proxy.RuntimeName(0), proxy.RuntimeName(3)} {
		id, ok := ParseRuntimeProxyID(name)
		require.True(t, ok)
		assert.Equal(t, uint(42), id)
	}
}

func TestParseRuntimeProxyIDRejectsDisplayAndBuiltinNames(t *testing.T) {
	for _, name := range []string{"same-name", "DIRECT", "[1]-same-name", "[pw:0]-same-name", "[pw:x]-same-name"} {
		_, ok := ParseRuntimeProxyID(name)
		assert.False(t, ok, name)
	}
}
