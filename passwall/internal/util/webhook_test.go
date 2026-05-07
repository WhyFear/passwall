package util

import (
	"encoding/json"
	"io"
	"net/http"
	"passwall/config"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookClient_ExecuteWebhooks(t *testing.T) {
	var gotMethod string
	var gotHeader string
	var gotBody map[string]string

	webhookClient := &WebhookClient{
		Client: &http.Client{
			Transport: webhookRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotMethod = r.Method
				gotHeader = r.Header.Get("X-Test")
				defer r.Body.Close()
				_ = json.NewDecoder(r.Body).Decode(&gotBody)

				return &http.Response{
					StatusCode: http.StatusAccepted,
					Status:     "202 Accepted",
					Body:       io.NopCloser(strings.NewReader("accepted")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}),
		},
	}
	webhooks := []config.WebhookConfig{
		{
			Name:   "webhook1",
			Method: "PUT",
			URL:    "https://example.test/webhook",
			Header: "X-Test: yes",
			Body:   `{"status":"{{status}}"}`,
		},
	}
	errs := webhookClient.ExecuteWebhooks(webhooks, map[string]interface{}{"status": "ok"})
	assert.Len(t, errs, 0)
	assert.Equal(t, http.MethodPut, gotMethod)
	assert.Equal(t, "yes", gotHeader)
	assert.Equal(t, "ok", gotBody["status"])
}

func TestWebhookClient_ExecuteWebhooksCollectsErrors(t *testing.T) {
	webhookClient := NewWebhookClient()
	errs := webhookClient.ExecuteWebhooks([]config.WebhookConfig{
		{Name: "bad", URL: ""},
	}, nil)

	assert.Len(t, errs, 1)
}

type webhookRoundTripFunc func(*http.Request) (*http.Response, error)

func (f webhookRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
