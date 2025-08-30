package risk

import (
	"passwall/internal/detector"
	"passwall/internal/util"

	"github.com/tidwall/gjson"
)

type NodeGetRiskDetector struct {
}

func NewNodeGetRiskDetector() Risk {
	return &NodeGetRiskDetector{}
}

func (n *NodeGetRiskDetector) Detect(ipProxy *detector.IPProxy) (*RiskResult, error) {
	// 0-100
	resp, err := util.GetUrl(ipProxy.ProxyClient, "https://ip.nodeget.com/json")
	if err != nil {
		return &RiskResult{
			Detector:   IPRiskDetectorNodeGet,
			Score:      -1,
			IPRiskType: n.GetRiskType(-1),
		}, nil
	}
	result := gjson.Parse(string(resp))
	score := result.Get("ip.riskScore").Int()

	return &RiskResult{
		Detector:   IPRiskDetectorNodeGet,
		Score:      int(score),
		IPRiskType: n.GetRiskType(score),
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
