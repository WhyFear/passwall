package util

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"
)

var UserAgent = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Edg/139.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/111.0",
}

// DownloadOptions 下载选项
type DownloadOptions struct {
	Timeout     time.Duration // 超时时间
	MaxFileSize int64         // 最大文件大小 (字节)
	ProxyURL    string        // 代理URL
}

// DefaultDownloadOptions 默认下载选项
var DefaultDownloadOptions = DownloadOptions{
	Timeout:     10 * time.Second, // 10秒超时
	MaxFileSize: 50 * 1024 * 1024, // 10MB最大大小
	ProxyURL:    "",               // 默认不使用代理
}

// DownloadFromURL 从URL下载内容
func DownloadFromURL(targetURL string, options *DownloadOptions) ([]byte, error) {
	return DownloadFromURLWithContext(context.Background(), targetURL, options)
}

// DownloadFromURLWithContext 从URL下载内容，并响应调用方取消
func DownloadFromURLWithContext(ctx context.Context, targetURL string, options *DownloadOptions) ([]byte, error) {
	if targetURL == "" {
		return nil, errors.New("URL cannot be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	resolvedOptions := DefaultDownloadOptions
	if options != nil {
		if options.Timeout > 0 {
			resolvedOptions.Timeout = options.Timeout
		}
		if options.MaxFileSize > 0 {
			resolvedOptions.MaxFileSize = options.MaxFileSize
		}
		resolvedOptions.ProxyURL = options.ProxyURL
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, resolvedOptions.Timeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	// 添加常用的请求头
	//req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	client, err := newRestrictedHTTPClient(resolvedOptions.Timeout, resolvedOptions.ProxyURL, net.DefaultResolver)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		// 检查是否是超时错误
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("request timed out after %s: %w", resolvedOptions.Timeout, context.DeadlineExceeded)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("HTTP request failed")
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP request failed with status: " + resp.Status)
	}

	// 限制读取大小
	limitReader := io.LimitReader(resp.Body, resolvedOptions.MaxFileSize)
	content, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	// 检查是否达到了大小限制
	if int64(len(content)) >= resolvedOptions.MaxFileSize {
		return nil, errors.New("content too large, exceeded maximum allowed size")
	}

	// 检查内容是否为空
	if len(content) == 0 {
		return nil, errors.New("downloaded content is empty")
	}

	return content, nil
}

func GetUrl(client *http.Client, url string) ([]byte, error) {
	return GetUrlWithContext(context.Background(), client, url)
}

func GetUrlWithContext(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return readHTTPResponse(resp, http.StatusOK)
}

func GetRandomUserAgent() string {
	return UserAgent[rand.Intn(len(UserAgent))]
}

func GetUrlWithHeaders(client *http.Client, url string, headers map[string]string) ([]byte, error) {
	return GetUrlWithHeadersContext(context.Background(), client, url, headers)
}

func GetUrlWithHeadersContext(ctx context.Context, client *http.Client, url string, headers map[string]string) ([]byte, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return readHTTPResponse(resp, http.StatusOK)
}

func PostUrlWithHeaders(client *http.Client, url string, headers map[string]string, body []byte) ([]byte, error) {
	return PostUrlWithHeadersContext(context.Background(), client, url, headers, body)
}

func PostUrlWithHeadersContext(ctx context.Context, client *http.Client, url string, headers map[string]string, body []byte) ([]byte, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return readHTTPResponse(resp, http.StatusOK, http.StatusCreated)
}

func readHTTPResponse(resp *http.Response, allowedStatuses ...int) ([]byte, error) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	statusAllowed := false
	for _, status := range allowedStatuses {
		if resp.StatusCode == status {
			statusAllowed = true
			break
		}
	}
	if !statusAllowed {
		return nil, errors.New("HTTP request failed with status: " + resp.Status)
	}

	// 检查是否为gzip压缩内容
	contentEncoding := resp.Header.Get("Content-Encoding")
	if contentEncoding == "gzip" {
		// 使用gzip reader解压内容
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		content, err := io.ReadAll(gzipReader)
		if err != nil {
			return nil, err
		}
		return content, nil
	}

	// 非压缩内容直接读取
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return content, nil
}
