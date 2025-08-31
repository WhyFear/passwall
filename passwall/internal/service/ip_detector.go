package service

import (
	"encoding/json"
	"passwall/internal/detector"
	"passwall/internal/detector/ipinfo"
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

type IPDetectorService interface {
	Detect(req *IPDetectorReq) error
	//GetInfo()
}

type ipDetectorImpl struct {
	Detector         *detector.DetectorManager
	ProxyIPAddress   repository.ProxyIPAddressRepository
	IPAddressRepo    repository.IPAddressRepository
	IPBaseInfoRepo   repository.IPBaseInfoRepository
	IPInfoRepo       repository.IPInfoRepository
	IPUnlockInfoRepo repository.IPUnlockInfoRepository
}

func NewIPDetector(ipAddressRepo repository.IPAddressRepository, ipBaseInfoRepo repository.IPBaseInfoRepository, ipInfoRepo repository.IPInfoRepository, ipUnlockInfoRepo repository.IPUnlockInfoRepository) IPDetectorService {
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
		log.Errorln("detect proxy ip failed, err: %v", err)
		return err
	}
	// 下面都是保存逻辑
	// ip address ，IPV6先不处理。
	ipAddressId := uint(0)
	if resp.BaseInfo.IPV4 != "" {
		ipAddress := &model.IPAddress{
			IP:     req.IPProxy.IP,
			IPType: 4,
		}
		err = i.IPAddressRepo.CreateOrIgnore(ipAddress)
		if err != nil {
			log.Errorln("create or update ip address failed, err: %v", err)
			return err
		}
		ipAddressId = ipAddress.ID

	}
	if ipAddressId == 0 {
		log.Infoln("ip address is empty, skip...")
		return nil
	}
	// proxy ip address
	err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
		ProxyID:       req.ProxyID,
		IPAddressesID: ipAddressId,
	})
	if err != nil {
		log.Errorln("create or update proxy ip address failed, err: %v", err)
		return err
	}
	// ip info, 顺便算一下base info的内容
	if resp.IPInfoResult != nil && len(resp.IPInfoResult) > 0 {
		ipInfoList := make([]*model.IPInfo, len(resp.IPInfoResult))
		riskLevelMap := make(map[ipinfo.IPRiskType]int)
		countryCodeMap := make(map[string]int)

		for _, ipInfo := range resp.IPInfoResult {
			if ipInfo.Risk.IPRiskType != ipinfo.IPRiskTypeDetectFailed {
				riskLevelMap[ipInfo.Risk.IPRiskType]++
			}
			if ipInfo.Geo.CountryCode != "" {
				countryCodeMap[ipInfo.Geo.CountryCode]++
			}
			riskJson, _ := json.Marshal(ipInfo.Risk)
			geoJson, _ := json.Marshal(ipInfo.Geo)
			ipInfoList = append(ipInfoList, &model.IPInfo{
				IPAddressesID: ipAddressId,
				Detector:      string(ipInfo.Detector),
				Risk:          riskJson,
				Geo:           geoJson,
				Raw:           ipInfo.Raw,
			})
		}
		err = i.IPInfoRepo.BatchCreateOrUpdate(ipInfoList)
		if err != nil {
			log.Errorln("create or update ip info failed, err: %v", err)
		}
		// ip base info 取出最大值
		var riskLevel ipinfo.IPRiskType
		var riskLevelCount int
		for k, v := range riskLevelMap {
			if v > riskLevelCount {
				riskLevel = k
				riskLevelCount = v
			}
		}
		var countryCode string
		var countryCodeCount int
		for k, v := range countryCodeMap {
			if v > countryCodeCount {
				countryCode = k
				countryCodeCount = v
			}
		}
		// ip base info
		ipBaseInfo := &model.IPBaseInfo{
			IPAddressesID: ipAddressId,
			RiskLevel:     string(riskLevel),
			CountryCode:   countryCode,
		}
		err = i.IPBaseInfoRepo.CreateOrUpdate(ipBaseInfo)
		if err != nil {
			log.Errorln("create or update ip base info failed, err: %v", err)
		}
	}
	// ip unlock info
	if resp.UnlockResult != nil && len(resp.UnlockResult) > 0 {
		ipUnlockInfoList := make([]*model.IPUnlockInfo, len(resp.UnlockResult))
		for i, unlockResult := range resp.UnlockResult {
			ipUnlockInfoList[i] = &model.IPUnlockInfo{
				IPAddressesID: ipAddressId,
				AppName:       string(unlockResult.APPName),
				Status:        string(unlockResult.Status),
				Region:        unlockResult.Region,
			}
		}
		err = i.IPUnlockInfoRepo.BatchCreateOrUpdate(ipUnlockInfoList)
		if err != nil {
			log.Errorln("create or update ip unlock info failed, err: %v", err)
		}
	}
	return nil
}
