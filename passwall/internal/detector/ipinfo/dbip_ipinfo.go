package ipinfo

import (
	"errors"
	"passwall/internal/model"
	"passwall/internal/util"
	"strings"

	"github.com/metacubex/mihomo/log"
	"github.com/tidwall/gjson"
)

type DBIPRiskDetector struct {
}

func NewDBIPRiskDetector() IPInfo {
	return &DBIPRiskDetector{}
}

func (i *DBIPRiskDetector) Detect(ipProxy *model.IPProxy) (*IPInfoResult, error) {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("DBIPRiskDetector Detect error: ipProxy is nil")
		return nil, errors.New("ipProxy is nil")
	}
	resp, err := util.GetUrl(ipProxy.ProxyClient, "https://db-ip.com/demo/home.php?s="+ipProxy.IP)
	if err != nil {
		return &IPInfoResult{
			Detector: DetectorDBIP,
			Risk: RiskResult{
				IPRiskType: IPRiskTypeDetectFailed,
			},
		}, nil
	}
	result := gjson.ParseBytes(resp)
	scoreText := result.Get("demoInfo.threatLevel").String()
	countryCode := strings.ToUpper(result.Get("demoInfo.countryCode").String())

	return &IPInfoResult{
		Detector: DetectorDBIP,
		Risk: RiskResult{
			ScoreText:  scoreText,
			IPRiskType: i.GetRiskType(scoreText),
		},
		Geo: IPGeoInfo{
			CountryCode: countryCode,
		},
		Raw: string(resp),
	}, nil
}

func (i *DBIPRiskDetector) GetRiskType(scoreText string) IPRiskType {
	switch scoreText {
	case "low":
		return IPRiskTypeLow
	case "medium":
		return IPRiskTypeMedium
	case "high":
		return IPRiskTypeHigh
	default:
		return IPRiskTypeDetectFailed
	}
}
