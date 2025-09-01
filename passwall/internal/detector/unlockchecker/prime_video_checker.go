package unlockchecker

import (
	"fmt"
	"log"
	"passwall/internal/model"
	"passwall/internal/util"
	"regexp"
	"strings"
)

type PrimeVideoUnlockCheck struct {
}

func NewPrimeVideoUnlockCheck() UnlockCheck {
	return &PrimeVideoUnlockCheck{}
}

func (p *PrimeVideoUnlockCheck) Check(ipProxy *model.IPProxy) *CheckResult {
	result := &CheckResult{
		APPName: PrimeVideo,
		Status:  CheckStatusFail,
		Region:  "",
	}

	// 检查IP代理是否有效
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		result.Status = CheckStatusFail
		result.Region = "no proxy"
		return result
	}

	// 设置请求头
	headers := map[string]string{
		"User-Agent":      util.GetRandomUserAgent(),
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language": "en-US,en;q=0.9",
		"Accept-Encoding": "gzip, deflate, br",
		"Connection":      "keep-alive",
	}

	// 发送请求到Prime Video
	url := "https://www.primevideo.com"
	content, err := util.GetUrlWithHeaders(ipProxy.ProxyClient, url, headers)
	if err != nil {
		log.Printf("PrimeVideo check request failed: %v", err)
		result.Status = CheckStatusFail
		result.Region = fmt.Sprintf("request error: %v", err)
		return result
	}

	// 解析响应内容
	contentStr := string(content)

	// 查找currentTerritory字段
	if strings.Contains(contentStr, "currentTerritory") {
		// 使用正则表达式匹配"currentTerritory":"XX"格式
		re := regexp.MustCompile(`"currentTerritory"\s*:\s*"([^"]+)"`)
		matches := re.FindStringSubmatch(contentStr)
		if len(matches) > 1 {
			region := matches[1]
			if region != "" {
				result.Status = CheckStatusUnlock
				result.Region = region
				return result
			}
		}
	}

	// 使用正则表达式进行更强大的匹配
	restrictedPatterns := []string{`not\s+available`, `unavailable`, `restricted`, `geoblocked`}
	for _, pattern := range restrictedPatterns {
		if regexp.MustCompile(`(?i)` + pattern).MatchString(contentStr) {
			result.Status = CheckStatusForbidden
			result.Region = "restricted"
			return result
		}
	}

	// 如果以上方法都失败，返回失败状态
	result.Status = CheckStatusFail
	result.Region = "no region info"
	return result
}
