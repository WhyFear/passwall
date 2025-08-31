package unlockchecker

import "passwall/internal/detector"

type application string

const (
	Unknown    application = "unknown"
	TikTok     application = "tiktok"
	DisneyPlus application = "disneyplus"
)

type checkStatus string

const (
	CheckStatusSuccess   checkStatus = "success"
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
	Check(ipProxy *detector.IPProxy) (*CheckResult, error)
}

type UnlockCheckFactory interface {
	RegisterUnlockChecker(detectorName application, checker UnlockCheck)
	GetUnlockChecker(detectorName application) (UnlockCheck, error)
	GetAllUnlockCheckers() []UnlockCheck
}
