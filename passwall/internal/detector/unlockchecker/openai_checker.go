package unlockchecker

import (
	"passwall/internal/model"
	"passwall/internal/util"
	"strings"

	"github.com/metacubex/mihomo/log"
)

type OpenAIUnlockCheck struct {
}

func NewOpenAIUnlockCheck() UnlockCheck {
	return &OpenAIUnlockCheck{}
}

func (o *OpenAIUnlockCheck) Check(ipProxy *model.IPProxy) *CheckResult {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("OpenAIUnlockCheck IPCheck error: ipProxy is nil")
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusFail,
		}
	}

	// 发送第一个请求到 api.openai.com
	tmpResult1, err1 := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://api.openai.com/compliance/cookie_requirements", map[string]string{
		"authority":          "api.openai.com",
		"accept":             "*/*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"authorization":      "Bearer null",
		"content-type":       "application/json",
		"origin":             "https://platform.openai.com",
		"referer":            "https://platform.openai.com/",
		"sec-ch-ua":          `"Microsoft Edge";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"user-agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",
	})

	// 发送第二个请求到 ios.chat.openai.com
	tmpResult2, err2 := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://ios.chat.openai.com/", map[string]string{
		"authority":                 "ios.chat.openai.com",
		"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"accept-language":           "zh-CN,zh;q=0.9",
		"sec-ch-ua":                 `"Microsoft Edge";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
		"sec-fetch-dest":            "document",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "none",
		"sec-fetch-user":            "?1",
		"upgrade-insecure-requests": "1",
		"user-agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",
	})

	// 初始化结果变量
	var result1, result2 string
	if err1 == nil {
		result1 = string(tmpResult1)
	}
	if err2 == nil {
		result2 = string(tmpResult2)
	}

	// 检查是否包含特定关键词
	containsUnsupportedCountry := strings.Contains(result1, "unsupported_country")
	containsVPN := strings.Contains(result2, "VPN")

	// 获取国家代码
	countryCode := ""
	countryResult, err := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://chat.openai.com/cdn-cgi/trace", nil)
	if err == nil {
		countryStr := string(countryResult)
		// 查找 "loc=" 开头的行
		lines := strings.Split(countryStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "loc=") {
				countryCode = strings.TrimPrefix(line, "loc=")
				break
			}
		}
	}

	// 根据结果判断解锁状态
	if !containsVPN && !containsUnsupportedCountry && err1 == nil && err2 == nil {
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusUnlock,
			Region:  countryCode,
		}
	} else if containsVPN && containsUnsupportedCountry {
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusForbidden,
			Region:  "",
		}
	} else if !containsUnsupportedCountry && containsVPN && err1 == nil {
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusUnlock,
			Region:  countryCode,
		}
	} else if containsUnsupportedCountry && !containsVPN {
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusUnlock,
			Region:  countryCode,
		}
	} else if err1 != nil && containsVPN {
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusForbidden,
			Region:  "",
		}
	} else {
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
