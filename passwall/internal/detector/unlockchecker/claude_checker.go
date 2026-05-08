package unlockchecker

import (
	MediaUnlockTest "MediaUnlockTest/checks"
	"context"
	"passwall/internal/model"
	"strings"
)

type ClaudeChecker struct {
}

func NewClaudeChecker() *ClaudeChecker {
	return &ClaudeChecker{}
}

func (c *ClaudeChecker) Check(ctx context.Context, ipProxy *model.IPProxy) *CheckResult {
	if ctx != nil && ctx.Err() != nil {
		return canceledCheckResult(Claude)
	}
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		return &CheckResult{
			APPName: Claude,
			Status:  CheckStatusFail,
		}
	}

	checkResult := MediaUnlockTest.Claude(*ipProxy.ProxyClient)
	if ctx != nil && ctx.Err() != nil {
		return canceledCheckResult(Claude)
	}
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
			Region:  strings.ToUpper(checkResult.Region),
		}
	default:
		return &CheckResult{
			APPName: Claude,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
