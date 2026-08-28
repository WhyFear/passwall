package util

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"passwall/config"
)

// WebhookClient webhook客户端
type WebhookClient struct {
	Client   *http.Client
	resolver outboundResolver
}

// NewWebhookClient 创建webhook客户端
func NewWebhookClient() *WebhookClient {
	client, _ := newRestrictedHTTPClient(30*time.Second, "", net.DefaultResolver)
	return &WebhookClient{
		Client:   client,
		resolver: net.DefaultResolver,
	}
}

// ExecuteWebhook 执行webhook请求
func (wc *WebhookClient) ExecuteWebhook(webhook config.WebhookConfig, data map[string]interface{}) error {
	if webhook.URL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	// 处理URL
	targetURL := webhook.URL

	// 处理请求体模板
	bodyContent := webhook.Body
	if bodyContent != "" && data != nil {
		// 简单的模板替换
		for key, value := range data {
			placeholder := fmt.Sprintf("{{%s}}", key)
			bodyContent = strings.ReplaceAll(bodyContent, placeholder, fmt.Sprintf("%v", value))
		}
	}

	// 创建请求
	var req *http.Request
	var err error

	method := strings.ToUpper(webhook.Method)
	if method == "" {
		method = "POST"
	}

	switch method {
	case "GET":
		// 对于GET请求，将参数添加到URL中
		if data != nil && len(data) > 0 {
			params := url.Values{}
			for key, value := range data {
				params.Add(key, fmt.Sprintf("%v", value))
			}
			if strings.Contains(targetURL, "?") {
				targetURL += "&" + params.Encode()
			} else {
				targetURL += "?" + params.Encode()
			}
		}
		req, err = http.NewRequest(method, targetURL, nil)
	case "POST", "PUT":
		// 对于POST和PUT等有body的方法
		var bodyBytes []byte
		if bodyContent != "" {
			bodyBytes = []byte(bodyContent)
		}
		req, err = http.NewRequest(method, targetURL, bytes.NewReader(bodyBytes))
	default:
		// 为其他HTTP方法留出扩展接口
		var bodyBytes []byte
		if bodyContent != "" {
			bodyBytes = []byte(bodyContent)
		}
		req, err = http.NewRequest(method, targetURL, bytes.NewReader(bodyBytes))
	}

	if err != nil {
		return fmt.Errorf("failed to create webhook request")
	}

	// 处理请求头
	if webhook.Header != "" {
		headers := strings.Split(webhook.Header, "\n")
		for _, header := range headers {
			if header == "" {
				continue
			}
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if key != "" && value != "" {
					req.Header.Set(key, value)
				}
			}
		}
	}

	// 设置默认Content-Type
	if method != "GET" && bodyContent != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req = req.WithContext(ctx)
	resolver := wc.resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if err := validateOutboundURL(ctx, req.URL, resolver); err != nil {
		return fmt.Errorf("webhook target is not allowed")
	}
	if wc.Client == nil {
		return fmt.Errorf("webhook client is unavailable")
	}
	client := *wc.Client
	redirectPolicy := restrictedRedirectPolicy(resolver)
	previousRedirectPolicy := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := redirectPolicy(req, via); err != nil {
			return err
		}
		if previousRedirectPolicy != nil {
			return previousRedirectPolicy(req, via)
		}
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook")
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP status %d", resp.StatusCode)
	}

	return nil
}

// ExecuteWebhooks 批量执行webhooks
func (wc *WebhookClient) ExecuteWebhooks(webhooks []config.WebhookConfig, data map[string]interface{}) []error {
	var errors []error

	for _, webhook := range webhooks {
		if err := wc.ExecuteWebhook(webhook, data); err != nil {
			errors = append(errors, fmt.Errorf("webhook %s failed: %v", webhook.Name, err))
		}
	}

	return errors
}
