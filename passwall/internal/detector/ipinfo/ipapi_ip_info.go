package ipinfo

import (
	"errors"
	"passwall/internal/model"
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

func (i *IPAPIRiskDetector) Detect(ipProxy *model.IPProxy) (*IPInfoResult, error) {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("IPAPIRiskDetector Detect error: ipProxy is nil")
		return nil, errors.New("ipProxy is nil")
	}

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
	if scoreText == "" {
		return &IPInfoResult{
			Detector: DetectorIPAPI,
			Risk: RiskResult{
				ScoreFloat: score,
				IPRiskType: i.GetRiskType(""),
			},
			Raw: string(resp),
		}, nil
	}
	// 空格分隔
	spaceIndex := strings.Index(scoreText, " ")
	// 处理超限问题
	if spaceIndex == -1 {
		return &IPInfoResult{
			Detector: DetectorIPAPI,
			Risk: RiskResult{
				ScoreFloat: score,
				IPRiskType: i.GetRiskType(""),
			},
			Raw: string(resp),
		}, nil
	}
	scoreString := scoreText[:spaceIndex]
	scoreDesc := scoreText[spaceIndex+1:]
	// 解析风险分数
	if len(scoreString) > 0 {
		score, err = strconv.ParseFloat(scoreString, 64)
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
			IPRiskType: i.GetRiskType(scoreDesc),
		},
		Geo: IPGeoInfo{
			CountryCode: countryCode,
		},
		Raw: string(resp),
	}, nil
}

func (i *IPAPIRiskDetector) GetRiskType(scoreText string) IPRiskType {
	switch scoreText {
	case "(Very Low)":
		return IPRiskTypeVeryLow
	case "(Low)":
		return IPRiskTypeLow
	case "(Elevated)":
		return IPRiskTypeMedium
	case "(High)":
		return IPRiskTypeHigh
	case "(Very High)":
		return IPRiskTypeVeryHigh
	default:
		return IPRiskTypeDetectFailed
	}
}
