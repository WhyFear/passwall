package handler

import (
	"encoding/base64"
	"net/http"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"strconv"

	"passwall/internal/adapter/generator"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ShareConfigHandler struct {
	shareConfigService service.ShareConfigService
}

func NewShareConfigHandler(shareConfigService service.ShareConfigService) *ShareConfigHandler {
	return &ShareConfigHandler{shareConfigService: shareConfigService}
}

func (h *ShareConfigHandler) List(c *gin.Context) {
	configs, err := h.shareConfigService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

func (h *ShareConfigHandler) Create(c *gin.Context) {
	var req service.ShareConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.shareConfigService.Create(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

func (h *ShareConfigHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share config id"})
		return
	}

	var req service.ShareConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.shareConfigService.Update(uint(id), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

func (h *ShareConfigHandler) Disable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share config id"})
		return
	}

	if err := h.shareConfigService.Disable(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Share config disabled successfully"})
}

func (h *ShareConfigHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share config id"})
		return
	}

	if err := h.shareConfigService.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Share config deleted successfully"})
}

func GetSharedSubscribe(shareConfigService service.ShareConfigService, proxyService proxy.ProxyService, generatorFactory generator.GeneratorFactory) gin.HandlerFunc {
	// ponytail: global limits avoid unbounded per-IP state; add trusted-proxy-aware buckets if legitimate traffic is throttled.
	limiter := rate.NewLimiter(1, 4)
	concurrent := make(chan struct{}, 4)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.Status(http.StatusTooManyRequests)
			return
		}
		select {
		case concurrent <- struct{}{}:
			defer func() { <-concurrent }()
		default:
			c.Status(http.StatusTooManyRequests)
			return
		}

		slug := c.Param("slug")
		if !validShareSlug(slug) {
			c.Status(http.StatusNotFound)
			return
		}
		config, err := shareConfigService.GetEnabledBySlug(slug)
		if err != nil {
			c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte(""))
			return
		}

		req := SubscribeReq{
			Type:        config.Type,
			StatusStr:   config.Status,
			ProxyType:   config.ProxyType,
			CountryCode: config.CountryCode,
			RiskLevel:   config.RiskLevel,
			AppUnlock:   config.AppUnlock,
			Sort:        config.Sort,
			SortOrder:   config.SortOrder,
			Limit:       config.Limit,
			WithIndex:   config.WithIndex,
		}

		content, err := GenerateSubscribeContent(req, proxyService, generatorFactory)
		if err != nil {
			writeSubscribeError(c, err)
			return
		}

		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	}
}

func validShareSlug(slug string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(slug)
	return err == nil && len(decoded) == 12 && base64.RawURLEncoding.EncodeToString(decoded) == slug
}
