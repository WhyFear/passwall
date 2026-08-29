package util

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"passwall/config"

	"github.com/stretchr/testify/require"
)

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	addresses, ok := r[host]
	if !ok {
		return nil, errors.New("host not found")
	}
	return addresses, nil
}

func TestDownloadRejectsLoopback(t *testing.T) {
	_, err := DownloadFromURL("http://127.0.0.1/subscription", nil)

	require.Error(t, err)
}

func TestWebhookRejectsLoopback(t *testing.T) {
	err := NewWebhookClient().ExecuteWebhook(config.WebhookConfig{
		Method: http.MethodPost,
		URL:    "http://127.0.0.1/webhook",
	}, nil)

	require.Error(t, err)
}

func TestValidateOutboundURLRejectsUnsafeTargets(t *testing.T) {
	tests := []string{
		"ftp://93.184.216.34/file",
		"http://user:password@93.184.216.34/file",
		"http://0.0.0.0/",
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://169.254.169.254/",
		"http://100.64.0.1/",
		"http://198.18.0.1/",
		"http://224.0.0.1/",
		"http://[::]/",
		"http://[::1]/",
		"http://[fc00::1]/",
		"http://[fe80::1]/",
		"http://[ff02::1]/",
		"http://[2001:db8::1]/",
		"http://[::ffff:127.0.0.1]/",
	}

	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			target, err := url.Parse(rawURL)
			require.NoError(t, err)
			require.Error(t, validateOutboundURL(context.Background(), target, staticResolver{}))
		})
	}
}

func TestValidateOutboundURLRejectsMixedDNS(t *testing.T) {
	target, err := url.Parse("https://mixed.test/subscription")
	require.NoError(t, err)

	err = validateOutboundURL(context.Background(), target, staticResolver{
		"mixed.test": {
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("10.0.0.1")},
		},
	})

	require.Error(t, err)
}

func TestValidateOutboundURLAllowsPublicHTTPAndHTTPS(t *testing.T) {
	for _, rawURL := range []string{
		"http://93.184.216.34/subscription",
		"https://8.8.8.8/subscription",
		"https://[2606:4700:4700::1111]/subscription",
	} {
		target, err := url.Parse(rawURL)
		require.NoError(t, err)
		require.NoError(t, validateOutboundURL(context.Background(), target, staticResolver{}))
	}
}

func TestProxyTransportValidatesTargetBeforeSending(t *testing.T) {
	target, err := http.NewRequest(http.MethodGet, "http://private.test/subscription", nil)
	require.NoError(t, err)
	var requests atomic.Int32
	transport := &restrictedRoundTripper{
		base: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			return nil, errors.New("unexpected request")
		}),
		resolver: staticResolver{
			"private.test": {{IP: net.ParseIP("10.0.0.1")}},
		},
		resolveTarget: true,
	}

	_, err = transport.RoundTrip(target)

	require.Error(t, err)
	require.Zero(t, requests.Load())
}

func TestRestrictedHTTPClientSetsTimeoutAndRedirectPolicy(t *testing.T) {
	client, err := newRestrictedHTTPClient(3*time.Second, "", staticResolver{})

	require.NoError(t, err)
	require.Equal(t, 3*time.Second, client.Timeout)
	require.NotNil(t, client.CheckRedirect)
}

func TestRestrictedClientRejectsPrivateRedirect(t *testing.T) {
	resolver := staticResolver{
		"public.test":  {{IP: net.ParseIP("93.184.216.34")}},
		"private.test": {{IP: net.ParseIP("10.0.0.1")}},
	}
	var requests atomic.Int32
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{
			StatusCode: http.StatusFound,
			Status:     "302 Found",
			Header:     http.Header{"Location": []string{"http://private.test/secret"}},
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Request:    r,
		}, nil
	})
	client := &http.Client{
		Timeout:       time.Second,
		Transport:     &restrictedRoundTripper{base: base, resolver: resolver, resolveTarget: true},
		CheckRedirect: restrictedRedirectPolicy(resolver),
	}

	resp, err := client.Get("http://public.test/start")
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Error(t, err)
	require.Equal(t, int32(1), requests.Load())
}

func TestRestrictedDialPinsValidatedIP(t *testing.T) {
	var dialed string
	dial := restrictedDialContext(staticResolver{
		"public.test": {{IP: net.ParseIP("93.184.216.34")}},
	}, func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = address
		return nil, errors.New("stop after capture")
	})

	_, err := dial(context.Background(), "tcp", "public.test:443")

	require.Error(t, err)
	require.Equal(t, "93.184.216.34:443", dialed)
}
