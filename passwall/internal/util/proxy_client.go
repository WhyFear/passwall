package util

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"passwall/internal/model"
	"strconv"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
)

func GetClashProxyClient(proxyConfig *model.Proxy, timeout time.Duration) *http.Client {
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
