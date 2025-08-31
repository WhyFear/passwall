package ipinfo

import (
	"fmt"
)

type DefaultRiskFactory struct {
	detectors map[DetectorName]IPInfo
}

func NewRiskFactory() IPInfoFactory {
	return &DefaultRiskFactory{
		detectors: make(map[DetectorName]IPInfo),
	}
}

func (f *DefaultRiskFactory) RegisterIPInfoDetector(detectorName DetectorName, risk IPInfo) {
	f.detectors[detectorName] = risk
}

func (f *DefaultRiskFactory) GetIPInfoDetector(detectorName DetectorName) (IPInfo, error) {
	detector, exists := f.detectors[detectorName]
	if !exists {
		return nil, fmt.Errorf("ipinfo detector not found for type: %s", detectorName)
	}
	return detector, nil
}

func (f *DefaultRiskFactory) GetAllIPInfoDetectors() []IPInfo {
	allDetectors := make([]IPInfo, 0, len(f.detectors))
	for _, detector := range f.detectors {
		allDetectors = append(allDetectors, detector)
	}
	return allDetectors
}
