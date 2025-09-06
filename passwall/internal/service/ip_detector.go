package service

import (
	"context"
	"encoding/json"
	"passwall/internal/detector"
	"passwall/internal/detector/ipinfo"
	"passwall/internal/model"
	"passwall/internal/repository"
	"strings"

	"github.com/metacubex/mihomo/log"
	"golang.org/x/sync/errgroup"
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
	Concurrent      int
}

type IPDetectResp struct {
	IPv4        string                `json:"ipv4"`
	IPv6        string                `json:"ipv6"`
	Risk        string                `json:"risk"`
	CountryCode string                `json:"country_code"`
	AppUnlock   []*model.IPUnlockInfo `json:"app_unlock"`
}

type IPDetectorService interface {
	BatchDetect(req *BatchIPDetectorReq) error
	Detect(req *IPDetectorReq) error
	GetInfo(req *IPDetectorReq) (*IPDetectResp, error)
	GetProxyIDsNotInIPAddress() ([]uint, error)
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
	if req.Concurrent == 0 {
		req.Concurrent = 20
	}
	eg, _ := errgroup.WithContext(context.Background())
	eg.SetLimit(req.Concurrent)

	for _, proxyID := range req.ProxyIDList {
		eg.Go(func() error {
			defer func() {
				if err := recover(); err != nil {
					log.Errorln("batch detect proxy ip failed, proxy id: %v, err: %v", proxyID, err)
				}
			}()
			err := i.Detect(&IPDetectorReq{
				ProxyID:         proxyID,
				Enabled:         true,
				IPInfoEnable:    req.IPInfoEnable,
				APPUnlockEnable: req.APPUnlockEnable,
				Refresh:         req.Refresh,
			})
			return err
		})
	}
	err := eg.Wait()
	if err != nil {
		log.Errorln("batch detect proxy ip failed, err: %v", err)
		return err
	}
	return nil
}

func (i ipDetectorImpl) Detect(req *IPDetectorReq) error {
	if !req.Enabled {
		log.Infoln("ip detector is disabled")
		return nil
	}
	// get proxy
	proxy, err := i.ProxyRepo.FindByID(req.ProxyID)
	if err != nil {
		log.Errorln("find proxy by id failed, err: %v", err)
		return err
	}
	if proxy == nil {
		log.Errorln("proxy is nil, skip...")
		return nil
	}
	req.IPProxy = model.NewIPProxy(proxy)

	if !req.Refresh {
		proxyIPAddress, err := i.ProxyIPAddress.FindByProxyID(req.ProxyID)
		if err != nil {
			log.Errorln("find proxy ip address by proxy id failed, err: %v", err)
			return err
		}
		if len(proxyIPAddress) > 0 {
			log.Infoln("refresh is disabled, have record, skip...")
			return nil
		}
		// 先获取ip地址，然后如果没有记录再做其他检测
		resp, err := i.Detector.DetectAll(req.IPProxy, false, false)
		if err != nil {
			log.Errorln("detect proxy ip failed, err: %v", err)
			return err
		}
		if resp.BaseInfo != nil {
			if resp.BaseInfo.IPV4 != "" {
				ipAddress, err := i.IPAddressRepo.FindByIP(resp.BaseInfo.IPV4)
				if err != nil {
					log.Errorln("find ipv4 address by ip failed, err: %v", err)
					return err
				}
				if ipAddress != nil {
					err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
						ProxyID:       req.ProxyID,
						IPAddressesID: ipAddress.ID,
						IPType:        4,
					})
					if err != nil {
						log.Errorln("create or update proxy ip address failed, err: %v", err)
						return err
					}
					log.Infoln("ip address by proxy ip is: %v", ipAddress)
					return nil
				}
			}
			if resp.BaseInfo.IPV6 != "" {
				ipAddress, err := i.IPAddressRepo.FindByIP(resp.BaseInfo.IPV6)
				if err != nil {
					log.Errorln("find ipv6 address by ip failed, err: %v", err)
					return err
				}
				if ipAddress != nil {
					err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
						ProxyID:       req.ProxyID,
						IPAddressesID: ipAddress.ID,
						IPType:        6,
					})
					if err != nil {
						log.Errorln("create or update proxy ip address failed, err: %v", err)
						return err
					}
					log.Infoln("ip address by proxy ip is: %v", ipAddress)
					return nil
				}
			}
		}
		// no ip address, continue
	}
	// below is refresh logic
	resp, err := i.Detector.DetectAll(req.IPProxy, req.IPInfoEnable, req.APPUnlockEnable)
	if err != nil {
		log.Errorln("detect proxy ip failed, err: %v", err)
		return err
	}
	// 下面都是保存逻辑
	ipAddressId4 := uint(0)
	ipAddressId6 := uint(0)
	if resp.BaseInfo.IPV4 != "" {
		ipAddress := &model.IPAddress{
			IP:     resp.BaseInfo.IPV4,
			IPType: 4,
		}
		err = i.IPAddressRepo.CreateOrIgnore(ipAddress)
		if err != nil {
			log.Errorln("create or update ip address failed, err: %v", err)
			return err
		}
		ipAddressId4 = ipAddress.ID
		// proxy ip address
		err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
			ProxyID:       req.ProxyID,
			IPAddressesID: ipAddress.ID,
			IPType:        4,
		})
		if err != nil {
			log.Errorln("create or update proxy ip address failed, err: %v", err)
			return err
		}
	}
	if resp.BaseInfo.IPV6 != "" {
		ipAddress := &model.IPAddress{
			IP:     resp.BaseInfo.IPV6,
			IPType: 6,
		}
		err = i.IPAddressRepo.CreateOrIgnore(ipAddress)
		if err != nil {
			log.Errorln("create or update ip address failed, err: %v", err)
			return err
		}
		ipAddressId6 = ipAddress.ID
		// proxy ip address
		err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
			ProxyID:       req.ProxyID,
			IPAddressesID: ipAddress.ID,
			IPType:        6,
		})
		if err != nil {
			log.Errorln("create or update proxy ip address failed, err: %v", err)
			return err
		}
	}
	if ipAddressId4 == 0 && ipAddressId6 == 0 {
		log.Infoln("ip address is empty, skip...")
		return nil
	}

	// IPInfoResultMap
	for ip, ipInfoResultList := range resp.IPInfoResultMap {
		ipAddressId := uint(0)
		if ip == resp.BaseInfo.IPV4 {
			ipAddressId = ipAddressId4
		}
		if ip == resp.BaseInfo.IPV6 {
			ipAddressId = ipAddressId6
		}
		if ipAddressId == 0 {
			log.Errorln("ip address id is empty, ip: %v", ip)
			continue
		}
		if ipInfoResultList != nil && len(ipInfoResultList) > 0 {
			ipInfoList := make([]*model.IPInfo, len(ipInfoResultList))
			riskLevelMap := make(map[ipinfo.IPRiskType]int)
			countryCodeMap := make(map[string]int)

			for _, ipInfo := range ipInfoResultList {
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
			// riskLevelMap和countryCodeMap都为空，就不保存
			if len(riskLevelMap) == 0 && len(countryCodeMap) == 0 {
				log.Infoln("ip base info is empty, skip...")
			} else {
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
		}
		// ip unlock info
		if resp.UnlockResult != nil && len(resp.UnlockResult) > 0 {
			ipUnlockInfoList := make([]*model.IPUnlockInfo, len(resp.UnlockResult))
			for i, unlockResult := range resp.UnlockResult {
				ipUnlockInfoList[i] = &model.IPUnlockInfo{
					IPAddressesID: ipAddressId,
					AppName:       string(unlockResult.APPName),
					Status:        string(unlockResult.Status),
					Region:        strings.ToUpper(unlockResult.Region),
				}
			}
			err = i.IPUnlockInfoRepo.BatchCreateOrUpdate(ipUnlockInfoList)
			if err != nil {
				log.Errorln("create or update ip unlock info failed, err: %v", err)
			}
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
	// 优先取IPV4结果
	var ipAddId uint
	for _, proxyIP := range proxyIPList {
		if proxyIP.IPType == 4 {
			ipAddId = proxyIP.IPAddressesID
			ipAddress, err := i.IPAddressRepo.FindByID(proxyIP.IPAddressesID)
			if err != nil {
				log.Errorln("find ip address by id failed, err: %v", err)
				return nil, err
			}
			if ipAddress.IPType == 4 {
				resp.IPv4 = ipAddress.IP
			}
		}
		if proxyIP.IPType == 6 {
			if ipAddId == 0 { // ipv4 first
				ipAddId = proxyIP.IPAddressesID
			}
			ipAddress, err := i.IPAddressRepo.FindByID(proxyIP.IPAddressesID)
			if err != nil {
				log.Errorln("find ip address by id failed, err: %v", err)
				return nil, err
			}
			if ipAddress.IPType == 6 {
				resp.IPv6 = ipAddress.IP
			}
		}
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

func (i ipDetectorImpl) GetProxyIDsNotInIPAddress() ([]uint, error) {
	proxyIDList, err := i.ProxyIPAddress.GetDistinctProxyIDs()
	if err != nil {
		log.Errorln("get distinct proxy id failed, err: %v", err)
		return nil, err
	}
	return i.ProxyRepo.FindNotInIDs(proxyIDList)
}
