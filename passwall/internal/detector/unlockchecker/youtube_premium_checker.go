package unlockchecker

import (
	"passwall/internal/model"
	"passwall/internal/util"
	"strings"

	"github.com/metacubex/mihomo/log"
)

type YoutubePremiumCheck struct {
}

func NewYoutubePremiumCheck() UnlockCheck {
	return &YoutubePremiumCheck{}
}

func (check *YoutubePremiumCheck) Check(ipProxy *model.IPProxy) *CheckResult {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("YoutubePremiumCheck IPCheck error: ipProxy is nil")
		return &CheckResult{
			APPName: YouTubePremium,
			Status:  CheckStatusFail,
		}
	}

	headers := map[string]string{
		"User-Agent":      util.GetRandomUserAgent(),
		"Accept-Language": "en",
		"Cookie":          "YSC=BiCUU3-5Gdk; CONSENT=YES+cb.20220301-11-p0.en+FX+700; GPS=1; VISITOR_INFO1_LIVE=4VwPMkB7W5A; PREF=tz=Asia.Shanghai; _gcl_au=1.1.1809531354.1646633279",
	}

	// 请求YouTube Premium页面
	result, err := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://www.youtube.com/premium", headers)
	if err != nil {
		return &CheckResult{
			APPName: YouTubePremium,
			Status:  CheckStatusFail,
		}
	}

	resultStr := string(result)

	// 检查是否是中国地区（包含www.google.cn）
	if strings.Contains(resultStr, "www.google.cn") {
		return &CheckResult{
			APPName: YouTubePremium,
			Status:  CheckStatusForbidden,
			Region:  "CN",
		}
	}

	// 检查是否显示"Premium is not available in your country"
	if strings.Contains(resultStr, "Premium is not available in your country") {
		return &CheckResult{
			APPName: YouTubePremium,
			Status:  CheckStatusForbidden,
			Region:  "",
		}
	}

	// 提取地区信息
	region := ""
	if start := strings.Index(resultStr, `"contentRegion":"`); start != -1 {
		start += len(`"contentRegion":"`)
		if end := strings.Index(resultStr[start:], `"`); end != -1 {
			region = resultStr[start : start+end]
		}
	}

	// 检查是否包含"ad-free"内容（表示Premium可用）
	if strings.Contains(resultStr, "ad-free") {
		if region != "" {
			return &CheckResult{
				APPName: YouTubePremium,
				Status:  CheckStatusUnlock,
				Region:  region,
			}
		} else {
			return &CheckResult{
				APPName: YouTubePremium,
				Status:  CheckStatusUnlock,
				Region:  "",
			}
		}
	}

	// 其他情况表示失败
	return &CheckResult{
		APPName: YouTubePremium,
		Status:  CheckStatusFail,
		Region:  "",
	}
}
