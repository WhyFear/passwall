package unlockchecker

import (
	MediaUnlockTest "MediaUnlockTest/checks"
	"passwall/internal/model"
	"strings"

	"github.com/metacubex/mihomo/log"
)

type DisneyPlusChecker struct {
}

func NewDisneyPlusChecker() UnlockCheck {
	return &DisneyPlusChecker{}
}

func (c *DisneyPlusChecker) Check(ipProxy *model.IPProxy) *CheckResult {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("DisneyPlusUnlockCheck IPCheck error: ipProxy is nil")
		return &CheckResult{
			APPName: DisneyPlus,
			Status:  CheckStatusFail,
		}
	}
	checkResult := MediaUnlockTest.DisneyPlus(*ipProxy.ProxyClient)
	//var (
	//	StatusOK         = 1
	//	StatusNetworkErr = -1
	//	StatusErr        = -2
	//	StatusRestricted = 2
	//	StatusNo         = 3
	//	StatusBanned     = 4
	//	StatusFailed     = 5
	//	StatusUnexpected = 6
	//)
	switch checkResult.Status {
	case 1:
		return &CheckResult{
			APPName: DisneyPlus,
			Status:  CheckStatusUnlock,
			Region:  strings.ToUpper(checkResult.Region),
		}
	case 3:
		return &CheckResult{
			APPName: DisneyPlus,
			Status:  CheckStatusForbidden,
			Region:  strings.ToUpper(checkResult.Region),
		}
	default:
		return &CheckResult{
			APPName: DisneyPlus,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
