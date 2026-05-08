package service

import (
	"context"
	"passwall/internal/detector"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

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
	GetDistinctCountryCode() ([]string, error)
}

type ipDetectorImpl struct {
	ConfigService    ConfigService
	ProxyRepo        repository.ProxyRepository
	ProxyIPAddress   repository.ProxyIPAddressRepository
	IPAddressRepo    repository.IPAddressRepository
	IPBaseInfoRepo   repository.IPBaseInfoRepository
	IPInfoRepo       repository.IPInfoRepository
	IPUnlockInfoRepo repository.IPUnlockInfoRepository
	TaskManager      task.TaskManager
	Persister        *ipDetectPersister
	detectOne        func(req *IPDetectorReq) error
}

func NewIPDetector(configService ConfigService,
	proxyRepo repository.ProxyRepository,
	proxyIPAddressRepo repository.ProxyIPAddressRepository,
	ipAddressRepo repository.IPAddressRepository,
	ipBaseInfoRepo repository.IPBaseInfoRepository,
	ipInfoRepo repository.IPInfoRepository,
	ipUnlockInfoRepo repository.IPUnlockInfoRepository,
	taskManager task.TaskManager,
) IPDetectorService {
	return &ipDetectorImpl{
		ConfigService:    configService,
		ProxyRepo:        proxyRepo,
		ProxyIPAddress:   proxyIPAddressRepo,
		IPAddressRepo:    ipAddressRepo,
		IPBaseInfoRepo:   ipBaseInfoRepo,
		IPInfoRepo:       ipInfoRepo,
		IPUnlockInfoRepo: ipUnlockInfoRepo,
		TaskManager:      taskManager,
		Persister:        newIPDetectPersister(ipAddressRepo, proxyIPAddressRepo, ipBaseInfoRepo, ipInfoRepo, ipUnlockInfoRepo),
	}
}

func (i ipDetectorImpl) getDetector() (*detector.DetectorManager, error) {
	cfg, err := i.ConfigService.GetConfig()
	if err != nil {
		return nil, err
	}
	return detector.NewDetectorManager(*cfg), nil
}

func (i ipDetectorImpl) BatchDetect(req *BatchIPDetectorReq) error {
	if req == nil || !req.Enabled {
		return nil
	}
	if req.Concurrent == 0 {
		req.Concurrent = 20
	}

	taskRun, success := task.StartRun(context.Background(), i.TaskManager, task.TaskTypeCheckIp, len(req.ProxyIDList))
	if !success {
		log.Errorln("start task failed, task type: %v", task.TaskTypeCheckIp)
		return nil
	}

	finishMessage := "batch detect proxy ip finished"
	defer func() {
		if recoverValue := recover(); recoverValue != nil {
			finishMessage = "batch detect proxy ip panic"
			log.Errorln("batch detect proxy ip panic: %v", recoverValue)
		}
		if contextMessage := task.MessageForContext(taskRun.Context()); contextMessage != "" {
			finishMessage = contextMessage
		}
		taskRun.Finish(finishMessage)
	}()

	eg, ctx := errgroup.WithContext(taskRun.Context())
	eg.SetLimit(req.Concurrent)

detectLoop:
	for _, proxyID := range req.ProxyIDList {
		select {
		case <-ctx.Done():
			break detectLoop
		default:
		}

		pid := proxyID
		eg.Go(func() error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			defer func() {
				if err := recover(); err != nil {
					log.Errorln("batch detect proxy ip failed, proxy id: %v, err: %v", pid, err)
				}
				taskRun.IncrementProgress("")
			}()
			err := i.detect(&IPDetectorReq{
				ProxyID:         pid,
				Enabled:         true,
				IPInfoEnable:    req.IPInfoEnable,
				APPUnlockEnable: req.APPUnlockEnable,
				Refresh:         req.Refresh,
			})
			return err
		})
	}
	_ = eg.Wait()
	log.Infoln("batch detect proxy ip finished")
	return nil
}

func (i ipDetectorImpl) detect(req *IPDetectorReq) error {
	if i.detectOne != nil {
		return i.detectOne(req)
	}
	return i.Detect(req)
}

func (i ipDetectorImpl) Detect(req *IPDetectorReq) error {
	log.Infoln("start to detect proxy ip, proxy id: %v", req.ProxyID)
	if !req.Enabled {
		log.Infoln("ip detector is disabled, proxy id: %v", req.ProxyID)
		return nil
	}
	// get proxy
	proxy, err := i.ProxyRepo.FindByID(req.ProxyID)
	if err != nil {
		log.Errorln("find proxy by id failed, proxy id: %v, err: %v", req.ProxyID, err)
		return err
	}
	if proxy == nil {
		log.Errorln("proxy is nil, proxy id: %v, skip...", req.ProxyID)
		return nil
	}
	req.IPProxy = model.NewIPProxy(proxy)

	if !req.Refresh {
		proxyIPAddress, err := i.ProxyIPAddress.FindByProxyID(req.ProxyID)
		if err != nil {
			log.Errorln("find proxy ip address by proxy id failed, proxy id: %v, err: %v", req.ProxyID, err)
			return err
		}
		if len(proxyIPAddress) > 0 {
			log.Infoln("refresh is disabled, have record, skip..., proxy id: %v", req.ProxyID)
			return nil
		}
		// 先获取ip地址，然后如果没有记录再做其他检测
		det, err := i.getDetector()
		if err != nil {
			log.Errorln("get detector failed: %v", err)
			return err
		}
		resp, err := det.DetectAll(req.IPProxy, false, false)
		if err != nil {
			log.Errorln("detect proxy ip failed, proxy id: %v, err: %v", req.ProxyID, err)
			return err
		}
		if resp.BaseInfo != nil {
			if resp.BaseInfo.IPV4 != "" {
				ipAddress, err := i.IPAddressRepo.FindByIP(resp.BaseInfo.IPV4)
				if err != nil {
					log.Errorln("find ipv4 address by ip failed, proxy id: %v, err: %v", req.ProxyID, err)
					return err
				}
				if ipAddress != nil {
					err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
						ProxyID:       req.ProxyID,
						IPAddressesID: ipAddress.ID,
						IPType:        4,
					})
					if err != nil {
						log.Errorln("create or update proxy ip address failed, proxy id: %v, err: %v", req.ProxyID, err)
						return err
					}
					log.Infoln("ip address by proxy ip is: %v, proxy id: %v", ipAddress, req.ProxyID)
					return nil
				}
			}
			if resp.BaseInfo.IPV6 != "" {
				ipAddress, err := i.IPAddressRepo.FindByIP(resp.BaseInfo.IPV6)
				if err != nil {
					log.Errorln("find ipv6 address by ip failed, proxy id: %v, err: %v", req.ProxyID, err)
					return err
				}
				if ipAddress != nil {
					err = i.ProxyIPAddress.CreateOrUpdate(&model.ProxyIPAddress{
						ProxyID:       req.ProxyID,
						IPAddressesID: ipAddress.ID,
						IPType:        6,
					})
					if err != nil {
						log.Errorln("create or update proxy ip address failed, proxy id: %v, err: %v", req.ProxyID, err)
						return err
					}
					log.Infoln("ip address by proxy ip is: %v, proxy id: %v", ipAddress, req.ProxyID)
					return nil
				}
			}
		}
		// no ip address, continue
	}
	// below is refresh logic
	det, err := i.getDetector()
	if err != nil {
		log.Errorln("get detector failed: %v", err)
		return err
	}
	resp, err := det.DetectAll(req.IPProxy, req.IPInfoEnable, req.APPUnlockEnable)
	if err != nil {
		log.Errorln("detect proxy ip failed, proxy id: %v, err: %v", req.ProxyID, err)
		return err
	}
	return i.Persister.Persist(req.ProxyID, resp)
}

func (i ipDetectorImpl) GetInfo(req *IPDetectorReq) (*IPDetectResp, error) {
	resp := &IPDetectResp{}
	proxyIPList, err := i.ProxyIPAddress.FindByProxyID(req.ProxyID)
	if err != nil {
		log.Errorln("find proxy ip address by proxy id failed, err: %v", err)
		return nil, err
	}
	if len(proxyIPList) == 0 {
		log.Infoln("get ip address is empty, skip..., proxy id: %v", req.ProxyID)
		return nil, nil
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

func (i ipDetectorImpl) GetDistinctCountryCode() ([]string, error) {
	result, err := i.IPBaseInfoRepo.GetDistinctCountryCode()
	if err != nil {
		log.Errorln("get distinct country code failed, err: %v", err)
		return nil, err
	}
	return result, nil
}
