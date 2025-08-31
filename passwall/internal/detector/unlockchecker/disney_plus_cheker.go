package unlockchecker

import (
	"encoding/json"
	"fmt"
	"passwall/internal/detector"
	"passwall/internal/util"

	"github.com/metacubex/mihomo/log"
)

type DisneyPlusChecker struct {
}

func NewDisneyPlusChecker() UnlockCheck {
	return &DisneyPlusChecker{}
}

func (c *DisneyPlusChecker) Check(ipProxy *detector.IPProxy) (*CheckResult, error) {
	client := ipProxy.ProxyClient

	// 检查ProxyClient是否为nil
	if client == nil {
		log.Errorln("DisneyPlusChecker Check: ProxyClient is nil")
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}

	headers := map[string]string{
		"User-Agent":    util.GetRandomUserAgent(),
		"Content-Type":  "application/json",
		"Authorization": "Bearer ZGlzbmV5JmJyb3dzZXImMS4wLjA.Cu56AgSfBTDag5NiRA81oLHkDZfu5L3CKadnefEAY84",
	}

	// 第一次请求：获取设备断言
	deviceAssertionBody := `{"deviceFamily":"browser","applicationRuntime":"chrome","deviceProfile":"windows","attributes":{}}`
	deviceAssertionResp, err := util.PostUrlWithHeaders(client, "https://disney.api.edge.bamgrid.com/devices", headers, []byte(deviceAssertionBody))
	if err != nil {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}

	var deviceAssertionRespData map[string]interface{}
	if err := json.Unmarshal(deviceAssertionResp, &deviceAssertionRespData); err != nil {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}
	assertion, ok := deviceAssertionRespData["assertion"].(string)
	if !ok {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}

	// 第二次请求：获取访问令牌
	grantBody := `grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Atoken-exchange&latitude=0&longitude=0&platform=browser&subject_token=` + assertion + `&subject_token_type=urn%3Abamtech%3Aparams%3Aoauth%3Atoken-type%3Adevice`
	grantResp, err := util.PostUrlWithHeaders(client, "https://disney.api.edge.bamgrid.com/token", headers, []byte(grantBody))
	if err != nil {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusForbidden}, nil
	}

	var grantRespData map[string]interface{}
	if err := json.Unmarshal(grantResp, &grantRespData); err != nil {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}
	// {
	//  "error" : "unauthorized_client",
	//  "error_description" : "forbidden-location"
	//}
	_, ok = grantRespData["error_description"].(string)
	if ok {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusForbidden}, nil
	}

	refreshToken, ok := grantRespData["refresh_token"].(string)
	if !ok {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusForbidden}, nil
	}
	headers["Authorization"] = "Bearer " + refreshToken

	// 使用与shell脚本相同的GraphQL查询，包含extensions中的session信息
	graphqlQuery := fmt.Sprintf(`{"query":"mutation refreshToken($input: RefreshTokenInput!) {refreshToken(refreshToken: $input) {activeSession {sessionId}}}","variables":{"input":{"refreshToken":"%s"}}}`, refreshToken)

	graphqlResp, err := util.PostUrlWithHeaders(client, "https://disney.api.edge.bamgrid.com/graph/v1/device/graphql", headers, []byte(graphqlQuery))
	if err != nil {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}

	var graphqlRespData map[string]interface{}
	if err := json.Unmarshal(graphqlResp, &graphqlRespData); err != nil {
		log.Errorln("DisneyPlusChecker Check: Failed to unmarshal graphqlResp, err:", err)
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
	}

	// 解析地区信息
	// 首先尝试从extensions.sdk.session获取location信息
	var countryCode string
	if extensions, ok := graphqlRespData["extensions"].(map[string]interface{}); ok {
		if sdk, ok := extensions["sdk"].(map[string]interface{}); ok {
			if session, ok := sdk["session"].(map[string]interface{}); ok {
				if sessionLocation, ok := session["location"].(map[string]interface{}); ok {
					countryCode, _ = sessionLocation["countryCode"].(string)
				}
			}
		}
	}

	// 如果extensions中没有找到countryCode，则从account.location获取
	var account map[string]interface{}
	if countryCode == "" {
		if data, ok := graphqlRespData["data"].(map[string]interface{}); ok {
			account = data
		} else if acc, ok := graphqlRespData["account"].(map[string]interface{}); ok {
			account = acc
		} else {
			return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
		}

		location, ok := account["location"].(map[string]interface{})
		if !ok {
			return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
		}
		countryCode, ok = location["countryCode"].(string)
		if !ok {
			return &CheckResult{APPName: DisneyPlus, Status: CheckStatusFail}, nil
		}
	}

	if account == nil {
		if data, ok := graphqlRespData["data"].(map[string]interface{}); ok {
			account = data
		} else if acc, ok := graphqlRespData["account"].(map[string]interface{}); ok {
			account = acc
		}
	}

	// 根据countryCode和区域支持信息判断解锁状态
	// 特殊处理日本地区
	if countryCode == "JP" {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusUnlock, Region: countryCode}, nil
	}

	// 尝试从extensions.sdk.session获取inSupportedLocation信息
	var inSupportedLocation bool
	if extensions, ok := graphqlRespData["extensions"].(map[string]interface{}); ok {
		if sdk, ok := extensions["sdk"].(map[string]interface{}); ok {
			if session, ok := sdk["session"].(map[string]interface{}); ok {
				if supported, ok := session["inSupportedLocation"].(bool); ok {
					inSupportedLocation = supported
				}
			}
		}
	}

	// 根据inSupportedLocation和countryCode判断最终状态
	if inSupportedLocation {
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusUnlock, Region: countryCode}, nil
	} else if countryCode != "" {
		// 区域不支持
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusForbidden, Region: countryCode}, nil
	} else {
		// 有区域信息但不支持
		return &CheckResult{APPName: DisneyPlus, Status: CheckStatusForbidden, Region: countryCode}, nil
	}
}
