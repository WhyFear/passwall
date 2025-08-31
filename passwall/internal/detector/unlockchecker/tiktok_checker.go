package unlockchecker

import (
	"passwall/internal/model"
	"passwall/internal/util"
	"regexp"

	"github.com/metacubex/mihomo/log"
)

type TikTokUnlockCheck struct {
}

func NewTikTokUnlockCheck() UnlockCheck {
	return &TikTokUnlockCheck{}
}

func (t *TikTokUnlockCheck) Check(ipProxy *model.IPProxy) *CheckResult {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("TikTokUnlockCheck IPCheck error: ipProxy is nil")
		return &CheckResult{
			APPName: TikTok,
			Status:  CheckStatusFail,
		}
	}

	headers := map[string]string{
		"User-Agent": util.GetRandomUserAgent(),
	}
	resp, err := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://www.tiktok.com/", headers)
	if err != nil {
		return &CheckResult{
			APPName: TikTok,
			Status:  CheckStatusFail,
		}
	}
	region := ""
	reList := regexp.MustCompile(`"region"\s*:\s*"([A-Z]{2})"`).FindStringSubmatch(string(resp))
	if len(reList) >= 2 {
		region = reList[1]
	}
	if region == "" {
		return &CheckResult{
			APPName: TikTok,
			Status:  CheckStatusFail,
		}
	}
	return &CheckResult{
		APPName: TikTok,
		Status:  CheckStatusUnlock,
		Region:  region,
	}
}
