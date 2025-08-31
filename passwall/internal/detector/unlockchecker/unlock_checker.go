package unlockchecker

import "passwall/internal/detector/model"

type application string

const (
	TikTok     application = "tiktok"
	DisneyPlus application = "disneyplus"
)

type checkStatus string

const (
	CheckStatusFail      checkStatus = "fail"
	CheckStatusUnlock    checkStatus = "unlock"
	CheckStatusForbidden checkStatus = "forbidden"
)

type CheckResult struct {
	APPName application // 服务商
	Status  checkStatus // 状态
	Region  string      // 区域
}

type UnlockCheck interface {
	Check(ipProxy *model.IPProxy) *CheckResult
}

type UnlockCheckFactory interface {
	RegisterUnlockChecker(detectorName application, checker UnlockCheck)
	GetUnlockChecker(detectorName application) (UnlockCheck, error)
	GetAllUnlockCheckers() []UnlockCheck
}
