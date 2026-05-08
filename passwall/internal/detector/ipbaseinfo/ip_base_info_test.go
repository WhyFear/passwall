package ipbaseinfo

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetProxyIP(t *testing.T) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var body string
			switch r.URL.Path {
			case "/v4":
				body = "198.51.100.24\n"
			case "/v6":
				body = `{"ip":"2001:db8::24"}`
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	originalServices := ipServices
	ipServices = []IPService{
		{Name: "TestIPv4A", URL: "https://example.test/v4"},
		{Name: "TestIPv4B", URL: "https://example.test/v4"},
		{Name: "TestIPv6", URL: "https://example.test/v6", Format: &IPFormat{Format: "json", IPPath: "ip"}},
	}
	t.Cleanup(func() {
		ipServices = originalServices
	})

	ipInfo, err := GetProxyIPWithContext(context.Background(), client)

	assert.NoError(t, err)
	assert.NotNil(t, ipInfo)
	assert.Equal(t, "198.51.100.24", ipInfo.IPV4)
	assert.Equal(t, "2001:db8::24", ipInfo.IPV6)
}

func TestGetProxyIPRejectsNilClient(t *testing.T) {
	ipInfo, err := GetProxyIPWithContext(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, ipInfo)
}

func TestGetProxyIPWithContextCancelsRequests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	requestStarted := make(chan struct{})
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			close(requestStarted)
			<-r.Context().Done()
			return nil, r.Context().Err()
		}),
	}

	originalServices := ipServices
	ipServices = []IPService{{Name: "Blocked", URL: "https://example.test/blocked"}}
	t.Cleanup(func() {
		ipServices = originalServices
	})

	done := make(chan error, 1)
	go func() {
		_, err := GetProxyIPWithContext(ctx, client)
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("GetProxyIPWithContext did not stop after context cancellation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGetAllProxyIPsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 测试实际调用外部URL
	ipInfo, err := GetProxyIPWithContext(context.Background(), client)

	if err != nil {
		t.Logf("获取IP失败: %v", err)
		t.Skip("网络连接失败，跳过测试")
	}

	assert.NotNil(t, ipInfo)
	t.Logf("测试结果 - IPv4: %s, IPv6: %s", ipInfo.IPV4, ipInfo.IPV6)
}
