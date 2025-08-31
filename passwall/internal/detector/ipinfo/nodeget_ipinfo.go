package ipinfo

import (
	"errors"
	"passwall/internal/model"
	"passwall/internal/util"
	"strings"

	"github.com/metacubex/mihomo/log"
	"github.com/tidwall/gjson"
)

type NodeGetRiskDetector struct {
}

func NewNodeGetRiskDetector() IPInfo {
	return &NodeGetRiskDetector{}
}

func (n *NodeGetRiskDetector) Detect(ipProxy *model.IPProxy) (*IPInfoResult, error) {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("NodeGetRiskDetector Detect error: ipProxy is nil")
		return nil, errors.New("ipProxy is nil")
	}
	// 0-100
	resp, err := util.GetUrl(ipProxy.ProxyClient, "https://ip.nodeget.com/json")
	if err != nil {
		return &IPInfoResult{
			Detector: DetectorNodeGet,
			Risk: RiskResult{
				Score:      -1,
				IPRiskType: n.GetRiskType(-1),
			},
		}, nil
	}
	result := gjson.Parse(string(resp))
	score := result.Get("ip.riskScore").Int()
	countryCode := strings.ToUpper(result.Get("ip.location.country").String())

	return &IPInfoResult{
		Detector: DetectorNodeGet,
		Risk: RiskResult{
			Score:      int(score),
			IPRiskType: n.GetRiskType(score),
		},
		Geo: IPGeoInfo{
			CountryCode: countryCode,
		},
		Raw: string(resp),
	}, nil
}

func (n *NodeGetRiskDetector) GetRiskType(score int64) IPRiskType {
	if score < 0 {
		return IPRiskTypeDetectFailed
	}
	if score < 10 {
		return IPRiskTypeVeryLow
	}
	if score < 15 {
		return IPRiskTypeLow
	}
	if score < 25 {
		return IPRiskTypeMedium
	}
	if score < 50 {
		return IPRiskTypeHigh
	}
	return IPRiskTypeVeryHigh
}
