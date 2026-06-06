package speedtester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"passwall/internal/model"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
	"golang.org/x/sync/errgroup"
)

const (
	defaultSpeedTestServerURL    = "https://speed.cloudflare.com"
	defaultSpeedTestDownloadSize = 50 * 1024 * 1024
	defaultSpeedTestUploadSize   = 20 * 1024 * 1024
	defaultSpeedTestTimeout      = 5 * time.Second
	defaultSpeedTestMaxLatency   = 1000 * time.Millisecond
	defaultSpeedTestConcurrent   = 5
)

// ClashCoreSpeedTester 是ClashCore测速器实现
type ClashCoreSpeedTester struct {
	serverURL string
}

// NewClashCoreSpeedTester 创建ClashCore测速器实例
func NewClashCoreSpeedTester() SpeedTester {
	return &ClashCoreSpeedTester{}
}

// Test	测试代理速度
func (t *ClashCoreSpeedTester) Test(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	clashProxy, serverURL, err := t.buildProxy(proxy)
	if err != nil {
		return nil, err
	}

	latency, packetLoss, err := testLatency(ctx, clashProxy, serverURL, defaultSpeedTestMaxLatency)
	if err != nil {
		return nil, err
	}

	result := &model.SpeedTestResult{Ping: int(latency.Milliseconds())}
	if packetLoss == 100 || latency > defaultSpeedTestMaxLatency {
		return result, nil
	}

	downloadSpeed, err := testDownloadSpeed(ctx, clashProxy, serverURL, defaultSpeedTestDownloadSize, defaultSpeedTestConcurrent, defaultSpeedTestTimeout)
	if err != nil {
		return nil, err
	}
	result.DownloadSpeed = int(downloadSpeed)
	if downloadSpeed <= 0 {
		return result, nil
	}

	uploadSpeed, err := testUploadSpeed(ctx, clashProxy, serverURL, defaultSpeedTestUploadSize, defaultSpeedTestConcurrent, defaultSpeedTestTimeout)
	if err != nil {
		return nil, err
	}
	result.UploadSpeed = int(uploadSpeed)
	return result, nil
}

// TestLatency 测试代理延迟
func (t *ClashCoreSpeedTester) TestLatency(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	clashProxy, serverURL, err := t.buildProxy(proxy)
	if err != nil {
		return nil, err
	}

	latency, packetLoss, err := testLatency(ctx, clashProxy, serverURL, defaultSpeedTestMaxLatency)
	if err != nil {
		return nil, err
	}

	result := &model.SpeedTestResult{Ping: int(latency.Milliseconds())}
	if packetLoss == 100 || latency > defaultSpeedTestMaxLatency {
		result.Error = fmt.Errorf("latency probe failed")
	}
	return result, nil
}

func (t *ClashCoreSpeedTester) buildProxy(proxy *model.Proxy) (constant.Proxy, string, error) {
	if proxy == nil {
		return nil, "", errors.New("proxy cannot be nil")
	}

	// 检查是否支持此代理类型
	supported := t.checkTesterSupport(proxy)
	if !supported {
		return nil, "", errors.New("unsupported proxy type: " + string(proxy.Type))
	}

	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(proxy.Config), &config); err != nil {
		return nil, "", errors.New("failed to parse proxy config: " + err.Error())
	}

	// 创建代理
	clashProxy, err := adapter.ParseProxy(config)
	if err != nil {
		return nil, "", fmt.Errorf("proxy %w", err)
	}

	serverURL := t.serverURL
	if serverURL == "" {
		serverURL = defaultSpeedTestServerURL
	}

	return clashProxy, serverURL, nil
}

func testLatency(ctx context.Context, proxy constant.Proxy, serverURL string, timeout time.Duration) (time.Duration, float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := createSpeedTestClient(proxy, timeout)
	latencies := make([]time.Duration, 0, 6)
	failedPings := 0

	for i := 0; i < 6; i++ {
		if err := sleepWithContext(ctx, 100*time.Millisecond); err != nil {
			return 0, 0, err
		}

		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/__down?bytes=0", serverURL), nil)
		if err != nil {
			return 0, 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, 0, ctx.Err()
			}
			failedPings++
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			latencies = append(latencies, time.Since(start))
		} else {
			failedPings++
		}
	}

	if len(latencies) == 0 {
		return 0, 100, nil
	}

	var sum time.Duration
	for _, latency := range latencies {
		sum += latency
	}
	avg := sum / time.Duration(len(latencies))
	packetLoss := float64(failedPings) / 6 * 100
	return avg, packetLoss, nil
}

func testDownloadSpeed(ctx context.Context, proxy constant.Proxy, serverURL string, totalSize, concurrent int, timeout time.Duration) (float64, error) {
	chunkSize := totalSize / concurrent
	if chunkSize <= 0 {
		return 0, nil
	}

	results := make(chan transferResult, concurrent)
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < concurrent; i++ {
		eg.Go(func() error {
			result, err := testDownload(ctx, proxy, serverURL, chunkSize, timeout)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return nil
			}
			results <- result
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return 0, err
	}
	close(results)
	return averageTransferSpeed(results), nil
}

func testUploadSpeed(ctx context.Context, proxy constant.Proxy, serverURL string, totalSize, concurrent int, timeout time.Duration) (float64, error) {
	chunkSize := totalSize / concurrent
	if chunkSize <= 0 {
		return 0, nil
	}

	results := make(chan transferResult, concurrent)
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < concurrent; i++ {
		eg.Go(func() error {
			result, err := testUpload(ctx, proxy, serverURL, chunkSize, timeout)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return nil
			}
			results <- result
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return 0, err
	}
	close(results)
	return averageTransferSpeed(results), nil
}

type transferResult struct {
	bytes    int64
	duration time.Duration
}

func testDownload(ctx context.Context, proxy constant.Proxy, serverURL string, size int, timeout time.Duration) (transferResult, error) {
	client := createSpeedTestClient(proxy, timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/__down?bytes=%d", serverURL, size), nil)
	if err != nil {
		return transferResult{}, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return transferResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return transferResult{}, fmt.Errorf("download request failed with status: %s", resp.Status)
	}

	downloadBytes, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return transferResult{}, err
	}
	return transferResult{bytes: downloadBytes, duration: time.Since(start)}, nil
}

func testUpload(ctx context.Context, proxy constant.Proxy, serverURL string, size int, timeout time.Duration) (transferResult, error) {
	client := createSpeedTestClient(proxy, timeout)
	body := bytes.NewReader(make([]byte, size))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/__up", serverURL), body)
	if err != nil {
		return transferResult{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return transferResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return transferResult{}, fmt.Errorf("upload request failed with status: %s", resp.Status)
	}

	return transferResult{bytes: int64(size), duration: time.Since(start)}, nil
}

func averageTransferSpeed(results <-chan transferResult) float64 {
	var totalBytes int64
	var totalDuration time.Duration
	var count int
	for result := range results {
		if result.duration <= 0 {
			continue
		}
		totalBytes += result.bytes
		totalDuration += result.duration
		count++
	}
	if count == 0 || totalDuration <= 0 {
		return 0
	}
	return float64(totalBytes) / (totalDuration / time.Duration(count)).Seconds()
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func createSpeedTestClient(proxy constant.Proxy, timeout time.Duration) *http.Client {
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
				return proxy.DialContext(ctx, &constant.Metadata{
					Host:    host,
					DstPort: u16Port,
				})
			},
		},
	}
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
