package model

import (
	"net/http"
	"passwall/internal/model"
	"passwall/internal/util"
	"time"
)

type IPProxy struct {
	IP          string
	ProxyClient *http.Client
}

func NewIPProxy(ip string, proxy *model.Proxy) *IPProxy {
	proxyClient := util.GetClashProxyClient(proxy, 5*time.Second)
	return &IPProxy{
		IP:          ip,
		ProxyClient: proxyClient,
	}
}
