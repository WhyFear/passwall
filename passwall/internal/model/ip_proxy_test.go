package model

import (
	"io"
	"net/http"
	"testing"
	"time"
)

func TestGetProxyClient(t *testing.T) {

	// 创建一个模拟的 ProxyClient 配置
	proxyConfig := &Proxy{
		Config: `{"your client here":true}`,
	}

	// 获取代理客户端
	client := GetClashProxyClient(proxyConfig, 5*time.Second)
	if client == nil {
		t.Fatal("Expected a valid http.Client, got nil")
	}

	// 发起请求
	resp, err := client.Get("https://icanhazip.com/")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// 验证响应状态码
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
	// 打印响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	t.Logf("Response body: %s", body)
}
