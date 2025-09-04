package unlockchecker

import (
	MediaUnlockTest "MediaUnlockTest/checks"
	"passwall/internal/model"
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
	checkResult := MediaUnlockTest.ChatGPT(*ipProxy.ProxyClient)
	switch checkResult.Status {
	case 1:
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusUnlock,
			Region:  strings.ToUpper(checkResult.Region),
		}
	case 3, 4:
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusForbidden,
			Region:  strings.ToUpper(checkResult.Region),
		}
	case 2:
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusRateLimit,
			Region:  strings.ToUpper(checkResult.Region),
		}
	default:
		return &CheckResult{
			APPName: OpenAI,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
