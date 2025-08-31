package service

import (
	"passwall/internal/detector"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/metacubex/mihomo/log"
)

type IPDetectorReq struct {
	ProxyID         uint
	Enabled         bool
	IPInfoEnable    bool
	APPUnlockEnable bool
	Refresh         bool
	IPProxy         *model.IPProxy
}

type IPDetector interface {
	Detect(req *IPDetectorReq) error
}

type ipDetectorImpl struct {
	Detector         *detector.DetectorManager
	ProxyIPAddress   repository.ProxyIPAddressRepository
	IPAddressRepo    repository.IPAddressRepository
	IPBaseInfoRepo   repository.IPBaseInfoRepository
	IPInfoRepo       repository.IPInfoRepository
	IPUnlockInfoRepo repository.IPUnlockInfoRepository
}

func NewIPDetector(ipAddressRepo repository.IPAddressRepository, ipBaseInfoRepo repository.IPBaseInfoRepository, ipInfoRepo repository.IPInfoRepository, ipUnlockInfoRepo repository.IPUnlockInfoRepository) IPDetector {
	return &ipDetectorImpl{
		Detector:         detector.NewDetectorManager(),
		IPAddressRepo:    ipAddressRepo,
		IPBaseInfoRepo:   ipBaseInfoRepo,
		IPInfoRepo:       ipInfoRepo,
		IPUnlockInfoRepo: ipUnlockInfoRepo,
	}
}

func (i ipDetectorImpl) Detect(req *IPDetectorReq) error {
	if !req.Enabled {
		log.Infoln("ip detector is disabled")
		return nil
	}
	// 先查数据库
	proxyIPAddress, err := i.ProxyIPAddress.FindByProxyID(req.ProxyID)
	if err != nil {
		log.Errorln("find proxy ip address by proxy id failed, err: %v", err)
		return err
	}
	if !req.Refresh && proxyIPAddress != nil {
		log.Infoln("refresh is disabled, have record, skip...")
		return nil
	}
	resp, err := i.Detector.DetectAll(req.IPProxy, req.IPInfoEnable, req.APPUnlockEnable)
	if err != nil {
		return err
	}
}
