package unlockchecker

import (
	MediaUnlockTest "MediaUnlockTest/checks"
	"passwall/internal/model"
	"strings"
)

type ClaudeChecker struct {
}

func NewClaudeChecker() *ClaudeChecker {
	return &ClaudeChecker{}
}

func (c *ClaudeChecker) Check(ipProxy *model.IPProxy) *CheckResult {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		return &CheckResult{
			APPName: Claude,
			Status:  CheckStatusFail,
		}
	}

	checkResult := MediaUnlockTest.Claude(*ipProxy.ProxyClient)
	switch checkResult.Status {
	case 1:
		return &CheckResult{
			APPName: Claude,
			Status:  CheckStatusUnlock,
			Region:  strings.ToUpper(checkResult.Region),
		}
	case 3:
		return &CheckResult{
			APPName: Claude,
			Status:  CheckStatusForbidden,
			Region:  checkResult.Region,
		}
	default:
		return &CheckResult{
			APPName: Claude,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
