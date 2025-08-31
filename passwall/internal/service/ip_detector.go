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

type BatchIPDetectorReq struct {
	ProxyIDList     []uint `json:"proxy_ids"`
	Enabled         bool
	IPInfoEnable    bool
	APPUnlockEnable bool
	Refresh         bool
}

type IPDetectResp struct {
	IPv4 string `json:"ipv4"`
	//IPv6        string `json:"ipv6"`
	Risk        string                `json:"risk"`
	CountryCode string                `json:"country_code"`
	AppUnlock   []*model.IPUnlockInfo `json:"app_unlock"`
}

type IPDetectorService interface {
	BatchDetect(req *BatchIPDetectorReq) error
	Detect(req *IPDetectorReq) error
	GetInfo(req *IPDetectorReq) (*IPDetectResp, error)
}

type ipDetectorImpl struct {
	Detector         *detector.DetectorManager
	ProxyRepo        repository.ProxyRepository
	ProxyIPAddress   repository.ProxyIPAddressRepository
	IPAddressRepo    repository.IPAddressRepository
	IPBaseInfoRepo   repository.IPBaseInfoRepository
	IPInfoRepo       repository.IPInfoRepository
	IPUnlockInfoRepo repository.IPUnlockInfoRepository
}

func NewIPDetector(proxyRepo repository.ProxyRepository, proxyIPAddressRepo repository.ProxyIPAddressRepository, ipAddressRepo repository.IPAddressRepository, ipBaseInfoRepo repository.IPBaseInfoRepository, ipInfoRepo repository.IPInfoRepository, ipUnlockInfoRepo repository.IPUnlockInfoRepository) IPDetectorService {
	return &ipDetectorImpl{
		Detector:         detector.NewDetectorManager(),
		ProxyRepo:        proxyRepo,
		ProxyIPAddress:   proxyIPAddressRepo,
		IPAddressRepo:    ipAddressRepo,
		IPBaseInfoRepo:   ipBaseInfoRepo,
		IPInfoRepo:       ipInfoRepo,
		IPUnlockInfoRepo: ipUnlockInfoRepo,
	}
}

func (i ipDetectorImpl) BatchDetect(req *BatchIPDetectorReq) error {
	if !req.Enabled {
		return nil
	}
	go func() {
		for _, proxyID := range req.ProxyIDList {
			_ = i.Detect(&IPDetectorReq{
				ProxyID:         proxyID,
				Enabled:         true,
				IPInfoEnable:    req.IPInfoEnable,
				APPUnlockEnable: req.APPUnlockEnable,
				Refresh:         req.Refresh,
			})
		}
	}()
	return nil
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
	if !req.Refresh && len(proxyIPAddress) > 0 {
		log.Infoln("refresh is disabled, have record, skip...")
		return nil
	}
	// 查代理
	proxy, err := i.ProxyRepo.FindByID(req.ProxyID)
	if err != nil {
		log.Errorln("find proxy by id failed, err: %v", err)
		return err
	}
	if proxy == nil {
		log.Errorln("proxy is nil, skip...")
		return nil
	}
	req.IPProxy = model.NewIPProxy("", proxy)
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

func (i ipDetectorImpl) GetInfo(req *IPDetectorReq) (*IPDetectResp, error) {
	resp := &IPDetectResp{}
	proxyIPList, err := i.ProxyIPAddress.FindByProxyID(req.ProxyID)
	if err != nil || len(proxyIPList) == 0 {
		log.Errorln("find proxy ip address by proxy id failed, err: %v", err)
		return nil, err
	}
	ipAddId := proxyIPList[0].IPAddressesID
	ipAddress, err := i.IPAddressRepo.FindByID(ipAddId)
	if err != nil {
		log.Errorln("find ip address by id failed, err: %v", err)
		return nil, err
	}
	if ipAddress.IPType == 4 {
		resp.IPv4 = ipAddress.IP
	}
	// 查ip base info
	ipBaseInfo, err := i.IPBaseInfoRepo.FindByIPAddressID(ipAddId)
	if err != nil {
		log.Errorln("find ip base info by ip failed, err: %v", err)
		return nil, err
	}
	if ipBaseInfo != nil {
		resp.CountryCode = ipBaseInfo.CountryCode
		resp.Risk = ipBaseInfo.RiskLevel
	}
	// 查app unlock info
	appUnlockList, err := i.IPUnlockInfoRepo.FindByIPAddressID(ipAddId)
	if err != nil {
		log.Errorln("find ip unlock info by ip failed, err: %v", err)
		return nil, err
	}
	if appUnlockList != nil {
		resp.AppUnlock = appUnlockList
	}
	return resp, nil
}
