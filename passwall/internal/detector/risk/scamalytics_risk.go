package risk

import (
	"passwall/internal/detector"
	"passwall/internal/util"
	"regexp"
	"strconv"

	"github.com/metacubex/mihomo/log"
)

type ScamalyticsRiskDetector struct {
}

func NewScamalyticsRiskDetector() Risk {
	return &ScamalyticsRiskDetector{}
}

func (s *ScamalyticsRiskDetector) Detect(ipProxy *detector.IPProxy) (*RiskResult, error) {
	resp, err := util.GetUrl(ipProxy.ProxyClient, "https://scamalytics.com/ip/"+ipProxy.IP)
	if err != nil {
		return &RiskResult{
			Detector:   IPRiskDetectorScamalytics,
			Score:      -1,
			IPRiskType: s.GetRiskType(-1),
		}, nil
	}
	// 从响应体中正则读取风险分数。local tmpscore=$(echo "$RESPONSE"|grep -oE 'Fraud Score: [0-9]+'|awk -F': ' '{print $2}')
	score := regexp.MustCompile(`Fraud Score: (\d+)`).FindStringSubmatch(string(resp))
	if len(score) < 2 {
		return &RiskResult{
			Detector:   IPRiskDetectorScamalytics,
			Score:      -1,
			IPRiskType: s.GetRiskType(-1),
		}, nil
	}
	// 转换分数
	scoreInt, err := strconv.Atoi(score[1])
	if err != nil {
		log.Warnln("Scamalytics risk detector: failed to convert score to int: %v", err)
		scoreInt = -1
	}
	return &RiskResult{
		Detector:   IPRiskDetectorScamalytics,
		Score:      scoreInt,
		IPRiskType: s.GetRiskType(scoreInt),
	}, nil
}

func (s *ScamalyticsRiskDetector) GetRiskType(score int) IPRiskType {
	if score < 0 {
		return IPRiskTypeDetectFailed
	}
	if score < 25 {
		return IPRiskTypeLow
	}
	if score < 50 {
		return IPRiskTypeMedium
	}
	if score < 75 {
		return IPRiskTypeHigh
	}
	return IPRiskTypeVeryHigh
}
