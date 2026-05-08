package service

import (
	"encoding/json"
	"strings"

	"passwall/internal/detector"
	"passwall/internal/detector/ipinfo"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/metacubex/mihomo/log"
)

type ipDetectPersister struct {
	ipAddressRepo    repository.IPAddressRepository
	proxyIPAddress   repository.ProxyIPAddressRepository
	ipBaseInfoRepo   repository.IPBaseInfoRepository
	ipInfoRepo       repository.IPInfoRepository
	ipUnlockInfoRepo repository.IPUnlockInfoRepository
}

func newIPDetectPersister(
	ipAddressRepo repository.IPAddressRepository,
	proxyIPAddress repository.ProxyIPAddressRepository,
	ipBaseInfoRepo repository.IPBaseInfoRepository,
	ipInfoRepo repository.IPInfoRepository,
	ipUnlockInfoRepo repository.IPUnlockInfoRepository,
) *ipDetectPersister {
	return &ipDetectPersister{
		ipAddressRepo:    ipAddressRepo,
		proxyIPAddress:   proxyIPAddress,
		ipBaseInfoRepo:   ipBaseInfoRepo,
		ipInfoRepo:       ipInfoRepo,
		ipUnlockInfoRepo: ipUnlockInfoRepo,
	}
}

func (p *ipDetectPersister) Persist(proxyID uint, resp *detector.DetectionResult) error {
	if resp.BaseInfo == nil {
		log.Warnln("ip base info is empty, proxy id: %v", proxyID)
		return nil
	}

	ipAddressIDByIP, err := p.persistBaseAddresses(proxyID, resp)
	if err != nil {
		return err
	}
	if len(ipAddressIDByIP) == 0 {
		log.Infoln("ip address is empty, skip..., proxy id: %v", proxyID)
		return nil
	}

	if resp.IPInfoResultMap == nil {
		log.Infoln("ip info result map is empty, proxy id: %v", proxyID)
		return nil
	}

	for ip, ipInfoResultList := range resp.IPInfoResultMap {
		ipAddressID := ipAddressIDByIP[ip]
		if ipAddressID == 0 {
			log.Errorln("ip address id is empty, proxy id: %v, ip: %v", proxyID, ip)
			continue
		}
		if err := p.persistIPInfo(proxyID, ipAddressID, ipInfoResultList); err != nil {
			return err
		}
		if err := p.persistUnlockInfo(proxyID, ipAddressID, resp); err != nil {
			return err
		}
	}
	return nil
}

func (p *ipDetectPersister) persistBaseAddresses(proxyID uint, resp *detector.DetectionResult) (map[string]uint, error) {
	result := make(map[string]uint)
	if resp.BaseInfo.IPV4 != "" {
		ipAddressID, err := p.persistAddress(proxyID, resp.BaseInfo.IPV4, 4)
		if err != nil {
			return nil, err
		}
		result[resp.BaseInfo.IPV4] = ipAddressID
	}
	if resp.BaseInfo.IPV6 != "" {
		ipAddressID, err := p.persistAddress(proxyID, resp.BaseInfo.IPV6, 6)
		if err != nil {
			return nil, err
		}
		result[resp.BaseInfo.IPV6] = ipAddressID
	}
	return result, nil
}

func (p *ipDetectPersister) persistAddress(proxyID uint, ip string, ipType uint) (uint, error) {
	ipAddress := &model.IPAddress{
		IP:     ip,
		IPType: ipType,
	}
	if err := p.ipAddressRepo.CreateOrIgnore(ipAddress); err != nil {
		log.Errorln("create or update ip address failed, proxy id: %v, err: %v", proxyID, err)
		return 0, err
	}
	if err := p.proxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
		ProxyID:       proxyID,
		IPAddressesID: ipAddress.ID,
		IPType:        ipType,
	}); err != nil {
		log.Errorln("create or update proxy ip address failed, proxy id: %v, err: %v", proxyID, err)
		return 0, err
	}
	return ipAddress.ID, nil
}

func (p *ipDetectPersister) persistIPInfo(proxyID uint, ipAddressID uint, ipInfoResultList []*ipinfo.IPInfoResult) error {
	if len(ipInfoResultList) == 0 {
		return nil
	}

	ipInfoList := make([]*model.IPInfo, 0, len(ipInfoResultList))
	riskLevelMap := make(map[ipinfo.IPRiskType]int)
	countryCodeMap := make(map[string]int)

	for _, ipInfo := range ipInfoResultList {
		if ipInfo.Risk.IPRiskType != ipinfo.IPRiskTypeDetectFailed {
			riskLevelMap[ipInfo.Risk.IPRiskType]++
		}
		if ipInfo.Geo.CountryCode != "" {
			countryCodeMap[ipInfo.Geo.CountryCode]++
		}
		riskJSON, _ := json.Marshal(ipInfo.Risk)
		geoJSON, _ := json.Marshal(ipInfo.Geo)
		ipInfoList = append(ipInfoList, &model.IPInfo{
			IPAddressesID: ipAddressID,
			Detector:      string(ipInfo.Detector),
			Risk:          riskJSON,
			Geo:           geoJSON,
			Raw:           ipInfo.Raw,
		})
	}
	if err := p.ipInfoRepo.BatchCreateOrUpdate(ipInfoList); err != nil {
		log.Errorln("create or update ip info failed, proxy id: %v, err: %v", proxyID, err)
		return err
	}
	return p.persistBaseInfo(proxyID, ipAddressID, riskLevelMap, countryCodeMap)
}

func (p *ipDetectPersister) persistBaseInfo(proxyID uint, ipAddressID uint, riskLevelMap map[ipinfo.IPRiskType]int, countryCodeMap map[string]int) error {
	if len(riskLevelMap) == 0 && len(countryCodeMap) == 0 {
		log.Infoln("ip base info is empty, skip..., proxy id: %v", proxyID)
		return nil
	}

	var riskLevel ipinfo.IPRiskType
	var riskLevelCount int
	for risk, count := range riskLevelMap {
		if count > riskLevelCount {
			riskLevel = risk
			riskLevelCount = count
		}
	}
	var countryCode string
	var countryCodeCount int
	for code, count := range countryCodeMap {
		if count > countryCodeCount {
			countryCode = code
			countryCodeCount = count
		}
	}
	if err := p.ipBaseInfoRepo.CreateOrUpdate(&model.IPBaseInfo{
		IPAddressesID: ipAddressID,
		RiskLevel:     string(riskLevel),
		CountryCode:   countryCode,
	}); err != nil {
		log.Errorln("create or update ip base info failed, proxy id: %v, err: %v", proxyID, err)
		return err
	}
	return nil
}

func (p *ipDetectPersister) persistUnlockInfo(proxyID uint, ipAddressID uint, resp *detector.DetectionResult) error {
	if len(resp.UnlockResult) == 0 {
		return nil
	}
	ipUnlockInfoList := make([]*model.IPUnlockInfo, len(resp.UnlockResult))
	for index, unlockResult := range resp.UnlockResult {
		ipUnlockInfoList[index] = &model.IPUnlockInfo{
			IPAddressesID: ipAddressID,
			AppName:       string(unlockResult.APPName),
			Status:        string(unlockResult.Status),
			Region:        strings.ToUpper(unlockResult.Region),
		}
	}
	if err := p.ipUnlockInfoRepo.BatchCreateOrUpdate(ipUnlockInfoList); err != nil {
		log.Errorln("create or update ip unlock info failed, proxy id: %v, err: %v", proxyID, err)
		return err
	}
	return nil
}
