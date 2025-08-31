package util

import (
	"passwall/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookClient_ExecuteWebhooks(t *testing.T) {
	webhookClient := NewWebhookClient()
	webhooks := []config.WebhookConfig{
		{
			Name:   "webhook1",
			Method: "PUT",
			URL:    "your url",
		},
	}
	errs := webhookClient.ExecuteWebhooks(webhooks, nil)
	assert.Len(t, errs, 0)
}
