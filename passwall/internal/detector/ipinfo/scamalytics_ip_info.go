package ipinfo

import (
	"passwall/internal/detector/model"
	"passwall/internal/util"
	"regexp"
	"strconv"

	"github.com/metacubex/mihomo/log"
)

type ScamalyticsRiskDetector struct {
}

func NewScamalyticsRiskDetector() IPInfo {
	return &ScamalyticsRiskDetector{}
}

func (s *ScamalyticsRiskDetector) Detect(ipProxy *model.IPProxy) (*IPInfoResult, error) {
	resp, err := util.GetUrl(ipProxy.ProxyClient, "https://scamalytics.com/ip/"+ipProxy.IP)
	if err != nil {
		return &IPInfoResult{
			Detector: DetectorScamalytics,
			Risk: RiskResult{
				Score:      -1,
				IPRiskType: s.GetRiskType(-1),
			},
		}, nil
	}
	// 从响应体中正则读取风险分数。local tmpscore=$(echo "$RESPONSE"|grep -oE 'Fraud Score: [0-9]+'|awk -F': ' '{print $2}')
	score := regexp.MustCompile(`Fraud Score: (\d+)`).FindStringSubmatch(string(resp))
	if len(score) < 2 {
		return &IPInfoResult{
			Detector: DetectorScamalytics,
			Risk: RiskResult{
				Score:      -1,
				IPRiskType: s.GetRiskType(-1),
			},
		}, nil
	}
	// 转换分数
	scoreInt, err := strconv.Atoi(score[1])
	if err != nil {
		log.Warnln("Scamalytics ipinfo detector: failed to convert score to int: %v", err)
		scoreInt = -1
	}
	countryCode := ""
	countryCodeList := regexp.MustCompile(`<th>Country Code<\/th><td>([A-Z]+)<\/td>`).FindStringSubmatch(string(resp))
	if len(countryCodeList) > 1 {
		// 转大写
		countryCode = countryCodeList[1]
	}

	return &IPInfoResult{
		Detector: DetectorScamalytics,
		Risk: RiskResult{
			Score:      scoreInt,
			IPRiskType: s.GetRiskType(scoreInt),
		},
		Geo: IPGeoInfo{
			CountryCode: countryCode,
		},
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
