package risk

import "passwall/internal/detector"

type RiskDetector struct {
}

type IPRiskDetector string

const (
	IPRiskDetectorUnknown     IPRiskDetector = "unknown"
	IPRiskDetectorScamalytics IPRiskDetector = "scamalytics"
	IPRiskDetectorIPAPI       IPRiskDetector = "ipapi"
	IPRiskDetectorNodeGet     IPRiskDetector = "nodeget"
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

type RiskResult struct {
	Detector   IPRiskDetector
	Score      int
	ScoreFloat float64
	ScoreText  string
	IPRiskType IPRiskType
}

type Risk interface {
	Detect(ipProxy *detector.IPProxy) (*RiskResult, error)
}

type RiskFactory interface {
	RegisterRiskDetector(typeName string, risk Risk)
	GetRiskDetector(typeName string) (Risk, error)
	GetAllRiskDetectors() []Risk
}
