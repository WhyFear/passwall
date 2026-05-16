package handler

import (
	"net/http"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"strconv"

	"passwall/internal/adapter/generator"

	"github.com/gin-gonic/gin"
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
	return func(c *gin.Context) {
		config, err := shareConfigService.GetEnabledBySlug(c.Param("slug"))
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
