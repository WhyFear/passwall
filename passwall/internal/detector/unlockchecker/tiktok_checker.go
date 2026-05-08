package unlockchecker

import (
	"context"
	"passwall/internal/model"
	"passwall/internal/util"
	"regexp"
	"strings"

	"github.com/metacubex/mihomo/log"
)

type TikTokUnlockCheck struct {
}

func NewTikTokUnlockCheck() UnlockCheck {
	return &TikTokUnlockCheck{}
}

func (t *TikTokUnlockCheck) Check(ctx context.Context, ipProxy *model.IPProxy) *CheckResult {
	if ctx != nil && ctx.Err() != nil {
		return canceledCheckResult(TikTok)
	}
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
	resp, err := util.GetUrlWithHeadersContext(ctx, ipProxy.ProxyClient, "https://www.tiktok.com/", headers)
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
		Region:  strings.ToUpper(region),
	}
}
