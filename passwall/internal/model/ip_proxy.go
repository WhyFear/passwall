package model

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
)

type IPProxy struct {
	IP          string
	IPV4        string
	IPV6        string
	ProxyClient *http.Client
}

func NewIPProxy(proxy *Proxy) *IPProxy {
	proxyClient := GetClashProxyClient(proxy, 5*time.Second)
	return &IPProxy{
		ProxyClient: proxyClient,
	}
}

func GetClashProxyClient(proxyConfig *Proxy, timeout time.Duration) *http.Client {
	// Config 转 map
	var configMap map[string]any
	if err := json.Unmarshal([]byte(proxyConfig.Config), &configMap); err != nil {
		return nil
	}
	clashProxy, err := adapter.ParseProxy(configMap)
	if err != nil {
		return nil
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				var u16Port uint16
				if port, err := strconv.ParseUint(port, 10, 16); err == nil {
					u16Port = uint16(port)
				}
				return clashProxy.DialContext(ctx, &constant.Metadata{
					Host:    host,
					DstPort: u16Port,
				})
			},
		},
	}
}
