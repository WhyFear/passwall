package unlockchecker

import (
	"encoding/json"
	"passwall/internal/model"
	"passwall/internal/util"
)

type SpotifyUnlockCheck struct {
}

func NewSpotifyUnlockCheck() UnlockCheck {
	return &SpotifyUnlockCheck{}
}

func (s *SpotifyUnlockCheck) Check(ipProxy *model.IPProxy) *CheckResult {
	// 检查IP代理是否有效
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		return &CheckResult{
			APPName: Spotify,
			Status:  CheckStatusFail,
		}
	}

	// 设置请求头
	headers := map[string]string{
		"User-Agent":      util.GetRandomUserAgent(),
		"Accept-Language": "en",
	}

	// 发送POST请求到Spotify注册接口
	url := "https://spclient.wg.spotify.com/signup/public/v1/account"
	body := "birth_day=11&birth_month=11&birth_year=2000&collect_personal_info=undefined&creation_flow=&creation_point=https%3A%2F%2Fwww.spotify.com%2Fhk-en%2F&displayname=Gay%20Lord&gender=male&iagree=1&key=a1e486e2729f46d6bb368d6b2bcda326&platform=www&referrer=&send-email=0&thirdpartyemail=0&identifier_token=AgE6YTvEzkReHNfJpO114514"

	resp, err := util.PostUrlWithHeaders(ipProxy.ProxyClient, url, headers, []byte(body))
	if err != nil {
		return &CheckResult{
			APPName: Spotify,
			Status:  CheckStatusFail,
		}
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return &CheckResult{
			APPName: Spotify,
			Status:  CheckStatusFail,
		}
	}

	// 获取响应中的字段
	statusCode, _ := result["status"].(float64)
	country, _ := result["country"].(string)
	isCountryLaunched, _ := result["is_country_launched"].(bool)

	// 根据响应内容判断解锁状态
	switch int(statusCode) {
	case 311:
		if isCountryLaunched {
			return &CheckResult{
				APPName: Spotify,
				Status:  CheckStatusUnlock,
				Region:  country,
			}
		} else {
			return &CheckResult{
				APPName: Spotify,
				Status:  CheckStatusForbidden,
				Region:  country,
			}
		}
	case 320, 120:
		return &CheckResult{
			APPName: Spotify,
			Status:  CheckStatusForbidden,
			Region:  "",
		}
	case 0:
		return &CheckResult{
			APPName: Spotify,
			Status:  CheckStatusFail,
			Region:  "",
		}
	default:
		return &CheckResult{
			APPName: Spotify,
			Status:  CheckStatusFail,
			Region:  "",
		}
	}
}
