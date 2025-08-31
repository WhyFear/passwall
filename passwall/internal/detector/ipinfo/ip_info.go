package ipinfo

import "passwall/internal/detector"

type IPInfoDetector struct {
}

type DetectorName string

const (
	DetectorUnknown     DetectorName = "unknown"
	DetectorScamalytics DetectorName = "scamalytics"
	DetectorIPAPI       DetectorName = "ipapi"
	DetectorNodeGet     DetectorName = "nodeget"
)

type IPRiskType string

const (
	IPRiskTypeDetectFailed IPRiskType = "detect_failed"
	IPRiskTypeVeryLow      IPRiskType = "very_low"
	IPRiskTypeLow          IPRiskType = "low"
	IPRiskTypeMedium       IPRiskType = "medium"
	IPRiskTypeHigh         IPRiskType = "high"
	IPRiskTypeVeryHigh     IPRiskType = "very_high"
)

type IPInfoResult struct {
	Detector DetectorName
	Risk     RiskResult `json:"ipinfo"`
	Geo      IPGeoInfo  `json:"geo"`
}

type RiskResult struct {
	Score      int
	ScoreFloat float64
	ScoreText  string
	IPRiskType IPRiskType
}

type IPGeoInfo struct {
	CountryCode string // should be uppercase
}

type IPInfo interface {
	Detect(ipProxy *detector.IPProxy) (*IPInfoResult, error)
}

type IPInfoFactory interface {
	RegisterIPInfoDetector(detectorName DetectorName, ipInfo IPInfo)
	GetIPInfoDetector(detectorName DetectorName) (IPInfo, error)
	GetAllIPInfoDetectors() []IPInfo
}
