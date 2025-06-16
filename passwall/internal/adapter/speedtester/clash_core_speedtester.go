package speedtester

import (
	"encoding/json"
	"errors"
	"fmt"
	"passwall/internal/model"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"

	"github.com/metacubex/mihomo/adapter"
)

// ClashCoreSpeedTester 是ClashCore测速器实现
type ClashCoreSpeedTester struct{}

// NewClashCoreSpeedTester 创建ClashCore测速器实例
func NewClashCoreSpeedTester() SpeedTester {
	return &ClashCoreSpeedTester{}
}

// Test	测试代理速度
func (t *ClashCoreSpeedTester) Test(proxy *model.Proxy) (*model.SpeedTestResult, error) {
	if proxy == nil {
		return nil, errors.New("proxy cannot be nil")
	}

	// 检查是否支持此代理类型
	supported := t.checkTesterSupport(proxy)
	if !supported {
		return nil, errors.New("unsupported proxy type: " + string(proxy.Type))
	}

	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(proxy.Config), &config); err != nil {
		return nil, errors.New("failed to parse proxy config: " + err.Error())
	}

	// 创建代理
	clashProxy, err := adapter.ParseProxy(config)
	if err != nil {
		return nil, fmt.Errorf("proxy %w", err)
	}
	allProxies := make(map[string]*speedtester.CProxy)
	allProxies[clashProxy.Name()] = &speedtester.CProxy{
		Proxy:  clashProxy,
		Config: config,
	}
	speedTester := speedtester.New(&speedtester.Config{
		ServerURL:        "https://speed.cloudflare.com",
		DownloadSize:     50 * 1024 * 1024,
		UploadSize:       20 * 1024 * 1024,
		Timeout:          time.Second * 5,
		MaxLatency:       1000 * time.Millisecond,
		MinDownloadSpeed: 0,
		MinUploadSpeed:   0,
		Concurrent:       5,
	})
	results := make([]*speedtester.Result, 0)
	speedTester.TestProxies(allProxies, func(result *speedtester.Result) {
		results = append(results, result)
	})

	// 返回测试结果
	if len(results) > 0 {
		return &model.SpeedTestResult{
			Ping:          int(results[0].Latency.Milliseconds()),
			DownloadSpeed: int(results[0].DownloadSpeed), // 转换为KB/s
			UploadSpeed:   int(results[0].UploadSpeed),   // 转换为KB/s
		}, nil
	}

	return nil, errors.New("测速失败：未获取到结果")
}

func (t *ClashCoreSpeedTester) checkTesterSupport(proxy *model.Proxy) bool {
	supported := false
	for _, supportedType := range t.SupportedTypes() {
		if proxy.Type == supportedType {
			supported = true
			break
		}
	}
	return supported
}

// SupportedTypes 返回支持的代理类型列表
func (t *ClashCoreSpeedTester) SupportedTypes() []model.ProxyType {
	return []model.ProxyType{
		model.ProxyTypeVMess,
		model.ProxyTypeVLess,
		model.ProxyTypeSS,
		model.ProxyTypeTrojan,
		model.ProxyTypeSocks5,
		model.ProxyTypeTuic,
		model.ProxyTypeSSR,
		model.ProxyTypeHysteria,
		model.ProxyTypeHysteria2,
		model.ProxyTypeWireGuard,
		model.ProxyTypeSnell,
		model.ProxyTypeHttp,
		model.ProxyTypeMieru,
		model.ProxyTypeAnyTLS,
		model.ProxyTypeSsh,
	}

}
