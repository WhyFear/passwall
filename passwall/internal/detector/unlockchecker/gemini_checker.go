package unlockchecker

import (
	MediaUnlockTest "MediaUnlockTest/checks"
	"context"
	"passwall/internal/model"
	"strings"

	"github.com/metacubex/mihomo/log"
)

type GeminiChecker struct {
}

func NewGeminiChecker() UnlockCheck {
	return &GeminiChecker{}
}

func (c *GeminiChecker) Check(ctx context.Context, ipProxy *model.IPProxy) *CheckResult {
	if ctx != nil && ctx.Err() != nil {
		return canceledCheckResult(Gemini)
	}
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("GeminiUnlockCheck IPCheck error: ipProxy is nil")
		return &CheckResult{
			APPName: Gemini,
			Status:  CheckStatusFail,
		}
	}
	checkResult := MediaUnlockTest.Gemini(*ipProxy.ProxyClient)
	if ctx != nil && ctx.Err() != nil {
		return canceledCheckResult(Gemini)
	}
	switch checkResult.Status {
	case 1:
		return &CheckResult{
			APPName: Gemini,
			Status:  CheckStatusUnlock,
			Region:  strings.ToUpper(checkResult.Region),
		}
	case 3, 4:
		return &CheckResult{
			APPName: Gemini,
			Status:  CheckStatusForbidden,
			Region:  strings.ToUpper(checkResult.Region),
		}
	default:
		return &CheckResult{
			APPName: Gemini,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
