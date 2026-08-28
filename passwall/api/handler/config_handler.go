package handler

import (
	"net/http"
	"passwall/config"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	configService service.ConfigService
}

type ConfigResponse struct {
	Concurrent int                                    `json:"concurrent"`
	Proxy      ProxyConfigResponse                    `json:"proxy"`
	IPCheck    IPCheckConfigResponse                  `json:"ip_check"`
	ClashAPI   ClashAPIConfigResponse                 `json:"clash_api"`
	CronJobs   []CronJobResponse                      `json:"cron_jobs"`
	DefaultSub config.DefaultSubscriptionUpdateConfig `json:"default_sub"`
}

type ProxyConfigResponse struct {
	Enabled       bool `json:"enabled"`
	URLConfigured bool `json:"url_configured"`
}

type IPCheckConfigResponse struct {
	Enable     bool                   `json:"enable"`
	IPInfo     IPInfoConfigResponse   `json:"ip_info"`
	AppUnlock  config.AppUnlockConfig `json:"app_unlock"`
	Refresh    bool                   `json:"refresh"`
	Concurrent int                    `json:"concurrent"`
}

type IPInfoConfigResponse struct {
	Enable      bool                      `json:"enable"`
	Scamalytics ScamalyticsConfigResponse `json:"scamalytics"`
}

type ScamalyticsConfigResponse struct {
	Configured bool `json:"configured"`
}

type ClashAPIConfigResponse struct {
	Enable  bool                     `json:"enable"`
	Clients []ClashAPIClientResponse `json:"clients"`
}

type ClashAPIClientResponse struct {
	ExistingIndex    int  `json:"existing_index"`
	URLConfigured    bool `json:"url_configured"`
	SecretConfigured bool `json:"secret_configured"`
}

type CronJobResponse struct {
	ExistingIndex int                     `json:"existing_index"`
	Name          string                  `json:"name"`
	Schedule      string                  `json:"schedule"`
	TestProxy     config.TestProxyConfig  `json:"test_proxy"`
	AutoBan       config.BanProxyConfig   `json:"auto_ban"`
	IPCheck       IPCheckConfigResponse   `json:"ip_check"`
	Webhook       []WebhookConfigResponse `json:"webhook"`
}

type WebhookConfigResponse struct {
	ExistingIndex    int    `json:"existing_index"`
	Name             string `json:"name"`
	Method           string `json:"method"`
	URLConfigured    bool   `json:"url_configured"`
	HeaderConfigured bool   `json:"header_configured"`
	BodyConfigured   bool   `json:"body_configured"`
}

func NewConfigHandler(configService service.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
	}
}

func (h *ConfigHandler) GetConfig(c *gin.Context) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configuration"})
		return
	}
	c.JSON(http.StatusOK, newConfigResponse(cfg))
}

func newConfigResponse(cfg *config.Config) ConfigResponse {
	clients := make([]ClashAPIClientResponse, len(cfg.ClashAPI.Clients))
	for i, client := range cfg.ClashAPI.Clients {
		clients[i] = ClashAPIClientResponse{
			ExistingIndex:    i,
			URLConfigured:    client.URL != "",
			SecretConfigured: client.Secret != "",
		}
	}
	cronJobs := make([]CronJobResponse, len(cfg.CronJobs))
	for i, job := range cfg.CronJobs {
		webhooks := make([]WebhookConfigResponse, len(job.Webhook))
		for j, webhook := range job.Webhook {
			webhooks[j] = WebhookConfigResponse{
				ExistingIndex:    j,
				Name:             webhook.Name,
				Method:           webhook.Method,
				URLConfigured:    webhook.URL != "",
				HeaderConfigured: webhook.Header != "",
				BodyConfigured:   webhook.Body != "",
			}
		}
		cronJobs[i] = CronJobResponse{
			ExistingIndex: i,
			Name:          job.Name,
			Schedule:      job.Schedule,
			TestProxy:     job.TestProxy,
			AutoBan:       job.AutoBan,
			IPCheck:       newIPCheckConfigResponse(job.IPCheck),
			Webhook:       webhooks,
		}
	}
	return ConfigResponse{
		Concurrent: cfg.Concurrent,
		Proxy: ProxyConfigResponse{
			Enabled:       cfg.Proxy.Enabled,
			URLConfigured: cfg.Proxy.URL != "",
		},
		IPCheck:    newIPCheckConfigResponse(cfg.IPCheck),
		ClashAPI:   ClashAPIConfigResponse{Enable: cfg.ClashAPI.Enable, Clients: clients},
		CronJobs:   cronJobs,
		DefaultSub: cfg.DefaultSub,
	}
}

func newIPCheckConfigResponse(cfg config.IPCheckConfig) IPCheckConfigResponse {
	return IPCheckConfigResponse{
		Enable: cfg.Enable,
		IPInfo: IPInfoConfigResponse{
			Enable:      cfg.IPInfo.Enable,
			Scamalytics: ScamalyticsConfigResponse{Configured: cfg.IPInfo.Scamalytics.Host != "" && cfg.IPInfo.Scamalytics.User != "" && cfg.IPInfo.Scamalytics.APIKey != ""},
		},
		AppUnlock:  cfg.AppUnlock,
		Refresh:    cfg.Refresh,
		Concurrent: cfg.Concurrent,
	}
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.configService.UpdateConfig(updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated successfully"})
}
