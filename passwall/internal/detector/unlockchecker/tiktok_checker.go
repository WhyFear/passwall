package unlockchecker

import (
	"passwall/internal/detector/model"
	"passwall/internal/util"
	"regexp"
)

type TikTokUnlockCheck struct {
}

func NewTikTokUnlockCheck() UnlockCheck {
	return &TikTokUnlockCheck{}
}

func (t *TikTokUnlockCheck) Check(ipProxy *model.IPProxy) *CheckResult {
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
