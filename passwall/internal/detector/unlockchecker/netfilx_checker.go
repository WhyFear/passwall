package unlockchecker

import (
	"passwall/internal/model"
	"passwall/internal/util"
	"regexp"
	"strings"

	"github.com/metacubex/mihomo/log"
)

type NetflixUnlockCheck struct {
}

func NewNetflixUnlockCheck() UnlockCheck {
	return &NetflixUnlockCheck{}
}

func (n *NetflixUnlockCheck) Check(ipProxy *model.IPProxy) *CheckResult {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("NetflixUnlockCheck IPCheck error: ipProxy is nil")
		return &CheckResult{
			APPName: Netflix,
			Status:  CheckStatusFail,
		}
	}

	headers := map[string]string{
		"User-Agent": util.GetRandomUserAgent(),
	}

	// 检查第一个Netflix视频
	result1, err := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://www.netflix.com/title/81280792", headers)
	if err != nil {
		log.Infoln("NetflixUnlockCheck GetUrlWithHeaders error: %v", err)
	}

	// 检查第二个Netflix视频
	result2, err := util.GetUrlWithHeaders(ipProxy.ProxyClient, "https://www.netflix.com/title/70143836", headers)
	if err != nil {
		log.Infoln("NetflixUnlockCheck GetUrlWithHeaders error: %v", err)
	}

	// 如果两个请求都失败
	if len(result1) == 0 || len(result2) == 0 {
		return &CheckResult{
			APPName: Netflix,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}

	// 提取地区信息
	region := ""
	reList := regexp.MustCompile(`data-country="([A-Z]{2})"`).FindStringSubmatch(string(result1))
	if len(reList) >= 2 {
		region = reList[1]
	}

	// 如果第一个请求没有找到地区信息，尝试第二个请求
	if region == "" {
		reList = regexp.MustCompile(`data-country="([A-Z]{2})"`).FindStringSubmatch(string(result2))
		if len(reList) >= 2 {
			region = reList[1]
		}
	}

	// 检查是否包含"Oh no!"内容（表示内容不可用）
	result1Str := string(result1)
	result2Str := string(result2)

	if contains(result1Str, "Oh no!") && contains(result2Str, "Oh no!") {
		return &CheckResult{
			APPName: Netflix,
			Status:  CheckStatusForbidden,
			Region:  strings.ToUpper(region),
		}
	}

	// 如果两个请求都成功且不包含"Oh no!"，表示解锁成功
	if !contains(result1Str, "Oh no!") && !contains(result2Str, "Oh no!") {
		return &CheckResult{
			APPName: Netflix,
			Status:  CheckStatusUnlock,
			Region:  strings.ToUpper(region),
		}
	}

	// 其他情况表示失败
	return &CheckResult{
		APPName: Netflix,
		Status:  CheckStatusFail,
		Region:  "",
	}
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
