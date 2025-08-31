package ipinfo

import (
	"passwall/internal/detector"
	"passwall/internal/util"
	"strconv"
	"strings"

	"github.com/metacubex/mihomo/log"
	"github.com/tidwall/gjson"
)

type IPAPIRiskDetector struct {
}

func NewIPAPIRiskDetector() IPInfo {
	return &IPAPIRiskDetector{}
}

func (i *IPAPIRiskDetector) Detect(ipProxy *detector.IPProxy) (*IPInfoResult, error) {
	score := -1.0
	resp, err := util.GetUrl(ipProxy.ProxyClient, "https://api.ipapi.is/?q="+ipProxy.IP)
	if err != nil {
		return &IPInfoResult{
			Detector: DetectorIPAPI,
			Risk: RiskResult{
				ScoreFloat: score,
				IPRiskType: i.GetRiskType(""),
			},
		}, nil
	}
	// 解析响应
	result := gjson.Parse(string(resp))
	// 提取风险分数, 格式为0.0067 (Low)
	scoreText := result.Get("company.abuser_score").String()
	// 空格分隔
	scoreTextList := strings.Split(scoreText, " ")
	if len(scoreTextList) > 0 {
		score, err = strconv.ParseFloat(scoreTextList[0], 64)
		if err != nil {
			log.Warnln("IPAPIRiskDetector Detect Atoi error: %v", err)
		}
	}
	// 提取国家代码
	countryCode := strings.ToUpper(result.Get("location.country_code").String())

	return &IPInfoResult{
		Detector: DetectorIPAPI,
		Risk: RiskResult{
			ScoreFloat: score,
			ScoreText:  scoreText,
			IPRiskType: i.GetRiskType(scoreTextList[1]),
		},
		Geo: IPGeoInfo{
			CountryCode: countryCode,
		},
	}, nil
}

func (i *IPAPIRiskDetector) GetRiskType(scoreText string) IPRiskType {
	switch scoreText {
	case "Very Low":
		return IPRiskTypeVeryLow
	case "Low":
		return IPRiskTypeLow
	case "Elevated":
		return IPRiskTypeMedium
	case "High":
		return IPRiskTypeHigh
	case "Very High":
		return IPRiskTypeVeryHigh
	default:
		return IPRiskTypeDetectFailed
	}
}
