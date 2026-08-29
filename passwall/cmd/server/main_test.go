package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHTTPServerSetsBoundaryTimeouts(t *testing.T) {
	server := newHTTPServer(":0", http.NewServeMux())

	assert.Equal(t, 5*time.Second, server.ReadHeaderTimeout)
	assert.Zero(t, server.ReadTimeout)
	assert.Equal(t, 60*time.Second, server.WriteTimeout)
	assert.Equal(t, 60*time.Second, server.IdleTimeout)
	assert.Equal(t, 1<<20, server.MaxHeaderBytes)
}
