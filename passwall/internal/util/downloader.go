package util

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DownloadOptions 下载选项
type DownloadOptions struct {
	Timeout     time.Duration // 超时时间
	MaxFileSize int64         // 最大文件大小 (字节)
	ProxyURL    string        // 代理URL
}

// DefaultDownloadOptions 默认下载选项
var DefaultDownloadOptions = DownloadOptions{
	Timeout:     10 * time.Second, // 10秒超时
	MaxFileSize: 10 * 1024 * 1024, // 10MB最大大小
	ProxyURL:    "",               // 默认不使用代理
}

// DownloadFromURL 从URL下载内容
func DownloadFromURL(targetURL string, options *DownloadOptions) ([]byte, error) {
	if targetURL == "" {
		return nil, errors.New("URL cannot be empty")
	}

	// 使用默认选项（如果未提供）
	if options == nil {
		options = &DefaultDownloadOptions
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	// 添加常用的请求头
	//req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	// 创建HTTP客户端
	client := &http.Client{}

	// 如果配置了代理，设置代理
	if options.ProxyURL != "" {
		proxyURLParsed, err := url.Parse(options.ProxyURL)
		if err != nil {
			return nil, errors.New("invalid proxy URL: " + err.Error())
		}

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP request failed with status: " + resp.Status)
	}

	// 限制读取大小
	limitReader := io.LimitReader(resp.Body, options.MaxFileSize)
	content, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	// 检查是否达到了大小限制
	if int64(len(content)) >= options.MaxFileSize {
		return nil, errors.New("content too large, exceeded maximum allowed size")
	}

	//// 清理HTML标签，特别是<pre>标签
	//contentStr := string(content)
	//if strings.Contains(contentStr, "<pre") {
	//	// 移除<pre>标签及其属性
	//	preTagRegex := regexp.MustCompile(`<pre[^>]*>`)
	//	contentStr = preTagRegex.ReplaceAllString(contentStr, "")
	//
	//	// 移除</pre>关闭标签
	//	contentStr = strings.ReplaceAll(contentStr, "</pre>", "")
	//}

	return content, nil
}
