package util

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadFromURLWithContextReturnsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DownloadFromURLWithContext(ctx, "https://example.test", &DownloadOptions{
		Timeout:     time.Minute,
		MaxFileSize: 1024,
	})

	require.Error(t, err)
}

func TestGetUrlWithContextUsesRequestContext(t *testing.T) {
	type contextKey string
	key := contextKey("marker")
	ctx := context.WithValue(context.Background(), key, "value")
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, "value", r.Context().Value(key))
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	body, err := GetUrlWithContext(ctx, client, "https://example.test")

	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), body)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
