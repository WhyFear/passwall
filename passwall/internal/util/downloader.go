package util

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

var UserAgent = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Edg/139.0.0.0",
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
	client := &http.Client{
		Timeout: options.Timeout,
	}

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
		// 检查是否是超时错误
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("request timed out after " + options.Timeout.String())
		}
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

	// 检查内容是否为空
	if len(content) == 0 {
		return nil, errors.New("downloaded content is empty")
	}

	return content, nil
}

func GetUrl(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
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

func GetRandomUserAgent() string {
	return UserAgent[rand.Intn(len(UserAgent))]
}

func GetUrlWithHeaders(client *http.Client, url string, headers map[string]string) ([]byte, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
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

func PostUrlWithHeaders(client *http.Client, url string, headers map[string]string, body []byte) ([]byte, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
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
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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
