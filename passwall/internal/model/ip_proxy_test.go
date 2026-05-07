package model

import (
	"testing"
	"time"
)

func TestGetClashProxyClientRejectsInvalidConfig(t *testing.T) {
	proxyConfig := &Proxy{
		Config: `{"invalid":true}`,
	}

	client := GetClashProxyClient(proxyConfig, 5*time.Second)
	if client != nil {
		t.Fatal("expected invalid clash config to return nil client")
	}
}

func TestNewIPProxyUsesConfigParser(t *testing.T) {
	ipProxy := NewIPProxy(&Proxy{Config: `{"invalid":true}`})
	if ipProxy == nil {
		t.Fatal("expected IPProxy wrapper")
	}
	if ipProxy.ProxyClient != nil {
		t.Fatal("expected invalid proxy config to produce nil HTTP client")
	}
}
