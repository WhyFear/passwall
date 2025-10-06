package ipinfo

import (
	"errors"
	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/util"

	"github.com/metacubex/mihomo/log"
	"github.com/tidwall/gjson"
)

type ScamalyticsRiskDetector struct {
	Config config.Scamalytics
}

func NewScamalyticsRiskDetector(cfg config.Scamalytics) IPInfo {
	return &ScamalyticsRiskDetector{
		Config: cfg,
	}
}

func (s *ScamalyticsRiskDetector) Detect(ipProxy *model.IPProxy) (*IPInfoResult, error) {
	if ipProxy == nil || ipProxy.ProxyClient == nil {
		log.Errorln("ScamalyticsRiskDetector Detect error: ipProxy is nil")
		return nil, errors.New("ipProxy is nil")
	}

	// 使用配置中的API信息,URL不通会返回404
	apiURL := s.Config.Host
	if s.Config.User != "" && s.Config.APIKey != "" {
		// 使用付费API
		apiURL = s.Config.Host + "/" + s.Config.User + "?key=" + s.Config.APIKey + "&ip=" + ipProxy.IP
	} else {
		log.Warnln("ScamalyticsRiskDetector Detect warn: user or apiKey is empty")
		return &IPInfoResult{
			Detector: DetectorScamalytics,
			Risk: RiskResult{
				IPRiskType: IPRiskTypeDetectFailed,
			},
		}, nil
	}

	resp, err := util.GetUrl(ipProxy.ProxyClient, apiURL)
	if err != nil {
		log.Warnln("ScamalyticsRiskDetector Detect error: %v", err)
		return &IPInfoResult{
			Detector: DetectorScamalytics,
			Risk: RiskResult{
				IPRiskType: IPRiskTypeDetectFailed,
			},
		}, nil
	}
	scoreInt := gjson.ParseBytes(resp).Get("scamalytics.scamalytics_score").Int()
	scoreText := gjson.ParseBytes(resp).Get("scamalytics.scamalytics_risk").String()
	// only ip2proxy_lite,maxmind_geolite2,ipinfo will return countryCode,choose first who not empty
	countryCode := gjson.ParseBytes(resp).Get("external_datasources.ip2proxy_lite.ip_country_code").String()
	if countryCode == "" {
		countryCode = gjson.ParseBytes(resp).Get("external_datasources.maxmind_geolite2.ip_country_code").String()
	}
	if countryCode == "" {
		countryCode = gjson.ParseBytes(resp).Get("external_datasources.ipinfo.ip_country_code").String()
	}

	return &IPInfoResult{
		Detector: DetectorScamalytics,
		Risk: RiskResult{
			Score:      int(scoreInt),
			ScoreText:  scoreText,
			IPRiskType: s.GetRiskType(scoreText),
		},
		Geo: IPGeoInfo{
			CountryCode: countryCode,
		},
		Raw: string(resp),
	}, nil
}

func (s *ScamalyticsRiskDetector) GetRiskType(scoreText string) IPRiskType {
	switch scoreText {
	case "low":
		return IPRiskTypeLow
	case "medium":
		return IPRiskTypeMedium
	case "high":
		return IPRiskTypeHigh
	case "very high":
		return IPRiskTypeVeryHigh
	default:
		return IPRiskTypeDetectFailed
	}
}
