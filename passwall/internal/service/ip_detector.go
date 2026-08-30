package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

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
	OnlyMissing     bool
}

type BatchIPDetectorReq struct {
	ProxyIDList     []uint `json:"proxy_ids"`
	Enabled         bool
	IPInfoEnable    bool
	APPUnlockEnable bool
	Refresh         bool
	Concurrent      int
	TaskResourceID  uint // 0 = global batch task; non-zero = resource-level task keyed by this ID
	Async           bool
}

type IPDetectResp struct {
	IPv4        string                `json:"ipv4"`
	IPv6        string                `json:"ipv6"`
	Risk        string                `json:"risk"`
	CountryCode string                `json:"country_code"`
	AppUnlock   []*model.IPUnlockInfo `json:"app_unlock"`
}

type IPDetectorService interface {
	BatchDetect(ctx context.Context, req *BatchIPDetectorReq) error
	DetectMissing(ctx context.Context, types []model.ProxyType, async bool) (int, error)
	Detect(ctx context.Context, req *IPDetectorReq) error
	GetInfo(req *IPDetectorReq) (*IPDetectResp, error)
	BatchGetInfo(proxyIDList []uint) (map[uint]*IPDetectResp, error)
	GetProxyIDsNotInIPAddress() ([]uint, error)
	GetDistinctCountryCode() ([]string, error)
}

type ipDetectTarget struct {
	ProxyID         uint
	IPInfoEnable    bool
	APPUnlockEnable bool
	Refresh         bool
	OnlyMissing     bool
	IPv4            string
	IPv6            string
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
	detectOne        func(ctx context.Context, req *IPDetectorReq) error
	detectAll        func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error)
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

func (i ipDetectorImpl) detectAllProxy(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
	if i.detectAll != nil {
		return i.detectAll(ctx, proxy, ipInfoEnable, appUnlockEnable)
	}
	det, err := i.getDetector()
	if err != nil {
		log.Errorln("get detector failed: %v", err)
		return nil, err
	}
	return det.DetectAll(ctx, proxy, ipInfoEnable, appUnlockEnable)
}

func (i ipDetectorImpl) BatchDetect(ctx context.Context, req *BatchIPDetectorReq) error {
	if req == nil || !req.Enabled {
		return nil
	}
	if req.Concurrent <= 0 {
		req.Concurrent = 20
	}
	targets := make([]ipDetectTarget, 0, len(req.ProxyIDList))
	for _, proxyID := range req.ProxyIDList {
		targets = append(targets, ipDetectTarget{
			ProxyID:         proxyID,
			IPInfoEnable:    req.IPInfoEnable,
			APPUnlockEnable: req.APPUnlockEnable,
			Refresh:         req.Refresh,
		})
	}
	return i.startBatchDetect(ctx, targets, req.Concurrent, req.TaskResourceID, req.Async)
}

func (i ipDetectorImpl) DetectMissing(ctx context.Context, types []model.ProxyType, async bool) (int, error) {
	cfg, err := i.ConfigService.GetConfig()
	if err != nil {
		return 0, err
	}
	ipCheck := cfg.IPCheck
	if !ipCheck.Enable || (!ipCheck.IPInfo.Enable && !ipCheck.AppUnlock.Enable) {
		return 0, nil
	}

	proxies, err := i.ProxyRepo.FindByFilter(&repository.NodeFilter{
		Status: []model.ProxyStatus{model.ProxyStatusOK},
		Types:  types,
	})
	if err != nil {
		return 0, fmt.Errorf("find proxies for missing ip detection: %w", err)
	}
	proxyIDs := make([]uint, 0, len(proxies))
	for _, proxy := range proxies {
		proxyIDs = append(proxyIDs, proxy.ID)
	}
	infoByProxyID, err := i.BatchGetInfo(proxyIDs)
	if err != nil {
		return 0, err
	}

	targets := make([]ipDetectTarget, 0, len(proxies))
	for _, proxy := range proxies {
		info := infoByProxyID[proxy.ID]
		missingIPInfo := ipCheck.IPInfo.Enable && (info == nil || strings.TrimSpace(info.Risk) == "" || strings.TrimSpace(info.CountryCode) == "")
		missingAppUnlock := ipCheck.AppUnlock.Enable && (info == nil || len(info.AppUnlock) == 0)
		if !missingIPInfo && !missingAppUnlock {
			continue
		}
		target := ipDetectTarget{
			ProxyID:         proxy.ID,
			IPInfoEnable:    missingIPInfo,
			APPUnlockEnable: missingAppUnlock,
			Refresh:         false,
			OnlyMissing:     true,
		}
		if info != nil {
			target.IPv4 = info.IPv4
			target.IPv6 = info.IPv6
		}
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		return 0, nil
	}
	if err := i.startBatchDetect(ctx, targets, ipCheck.Concurrent, 0, async); err != nil {
		return 0, err
	}
	return len(targets), nil
}

func (i ipDetectorImpl) startBatchDetect(ctx context.Context, targets []ipDetectTarget, concurrent int, taskResourceID uint, async bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if concurrent <= 0 {
		concurrent = 20
	}

	taskRun, success := task.StartRunWithSpec(ctx, i.TaskManager, task.TaskSpec{
		Type:       task.TaskTypeCheckIp,
		ResourceID: taskResourceID,
		Total:      len(targets),
		Accesses: []task.TaskAccess{
			{Resource: task.ResourceProxies, Mode: task.AccessModeRead},
			{Resource: task.ResourceIPDetection, Mode: task.AccessModeWrite, ResourceID: taskResourceID},
		},
	})
	if !success {
		log.Errorln("start task failed, task type: %v", task.TaskTypeCheckIp)
		return task.ErrTaskConflict
	}
	if async {
		go i.runBatchDetect(taskRun, targets, concurrent)
		return nil
	}
	i.runBatchDetect(taskRun, targets, concurrent)
	return nil
}

func (i ipDetectorImpl) runBatchDetect(taskRun *task.TaskRun, targets []ipDetectTarget, concurrent int) {
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
	eg.SetLimit(concurrent)
	var failureCount atomic.Int32
	var firstFailure string
	var firstFailureOnce sync.Once

detectLoop:
	for _, target := range targets {
		select {
		case <-ctx.Done():
			break detectLoop
		default:
		}

		target := target
		eg.Go(func() error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			defer func() {
				if err := recover(); err != nil {
					log.Errorln("batch detect proxy ip failed, proxy id: %v, err: %v", target.ProxyID, err)
					failureCount.Add(1)
				}
				taskRun.IncrementProgress("")
			}()
			err := i.detect(ctx, &IPDetectorReq{
				ProxyID:         target.ProxyID,
				Enabled:         true,
				IPInfoEnable:    target.IPInfoEnable,
				APPUnlockEnable: target.APPUnlockEnable,
				Refresh:         target.Refresh,
				OnlyMissing:     target.OnlyMissing,
				IPProxy: &model.IPProxy{
					IPV4: target.IPv4,
					IPV6: target.IPv6,
				},
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				failureCount.Add(1)
				firstFailureOnce.Do(func() {
					firstFailure = fmt.Sprintf("proxy id: %v, err: %v", target.ProxyID, err)
				})
			}
			return nil
		})
	}
	_ = eg.Wait()
	if n := failureCount.Load(); n > 0 {
		finishMessage = fmt.Sprintf("batch detect proxy ip finished, %d failure(s)", n)
		if firstFailure != "" {
			log.Warnln("%s, first failure: %s", finishMessage, firstFailure)
		} else {
			log.Warnln("%s", finishMessage)
		}
	} else {
		finishMessage = "batch detect proxy ip finished"
		log.Infoln("%s", finishMessage)
	}
}

func (i ipDetectorImpl) detect(ctx context.Context, req *IPDetectorReq) error {
	if i.detectOne != nil {
		return i.detectOne(ctx, req)
	}
	return i.Detect(ctx, req)
}

func (i ipDetectorImpl) Detect(ctx context.Context, req *IPDetectorReq) error {
	log.Infoln("start to detect proxy ip, proxy id: %v", req.ProxyID)
	if !req.Enabled {
		log.Infoln("ip detector is disabled, proxy id: %v", req.ProxyID)
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return ctx.Err()
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
	knownIPv4, knownIPv6 := "", ""
	if req.IPProxy != nil {
		knownIPv4, knownIPv6 = req.IPProxy.IPV4, req.IPProxy.IPV6
	}
	req.IPProxy = model.NewIPProxy(proxy)
	req.IPProxy.IPV4, req.IPProxy.IPV6 = knownIPv4, knownIPv6

	if !req.Refresh && !req.OnlyMissing {
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
		resp, err := i.detectAllProxy(ctx, req.IPProxy, false, false)
		if err != nil {
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
	resp, err := i.detectAllProxy(ctx, req.IPProxy, req.IPInfoEnable, req.APPUnlockEnable)
	if err != nil {
		return err
	}
	if req.OnlyMissing {
		return i.Persister.PersistMissing(req.ProxyID, resp)
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

// BatchGetInfo 批量获取代理ip信息，不会返回AppUnlock结果
func (i ipDetectorImpl) BatchGetInfo(proxyIDList []uint) (map[uint]*IPDetectResp, error) {
	result := make(map[uint]*IPDetectResp)
	if len(proxyIDList) == 0 {
		return result, nil
	}

	proxyIPList, err := i.ProxyIPAddress.FindLatestByProxyIDList(proxyIDList)
	if err != nil {
		log.Errorln("find latest proxy ip address by proxy ids failed, err: %v", err)
		return nil, err
	}

	selectedBaseInfoByProxyID := make(map[uint]*model.IPBaseInfo)
	ipAddressIDsByProxyID := make(map[uint][]uint)
	seenIPByProxyID := make(map[uint]map[uint]bool)
	for _, proxyIP := range proxyIPList {
		resp := result[proxyIP.ProxyID]
		if resp == nil {
			resp = &IPDetectResp{}
			result[proxyIP.ProxyID] = resp
		}
		if seenIPByProxyID[proxyIP.ProxyID] == nil {
			seenIPByProxyID[proxyIP.ProxyID] = make(map[uint]bool)
		}
		if !seenIPByProxyID[proxyIP.ProxyID][proxyIP.IPAddressesID] {
			seenIPByProxyID[proxyIP.ProxyID][proxyIP.IPAddressesID] = true
			ipAddressIDsByProxyID[proxyIP.ProxyID] = append(ipAddressIDsByProxyID[proxyIP.ProxyID], proxyIP.IPAddressesID)
		}

		ipAddress := proxyIP.IPAddress
		if proxyIP.IPType == 4 || ipAddress.IPType == 4 {
			resp.IPv4 = ipAddress.IP
			if ipAddress.IPBaseInfo.ID != 0 {
				selectedBaseInfoByProxyID[proxyIP.ProxyID] = &ipAddress.IPBaseInfo
			}
			continue
		}

		if proxyIP.IPType == 6 || ipAddress.IPType == 6 {
			resp.IPv6 = ipAddress.IP
			if selectedBaseInfoByProxyID[proxyIP.ProxyID] == nil && ipAddress.IPBaseInfo.ID != 0 {
				selectedBaseInfoByProxyID[proxyIP.ProxyID] = &ipAddress.IPBaseInfo
			}
		}
	}

	for proxyID, ipBaseInfo := range selectedBaseInfoByProxyID {
		if ipBaseInfo == nil {
			continue
		}
		if resp := result[proxyID]; resp != nil {
			resp.CountryCode = ipBaseInfo.CountryCode
			resp.Risk = ipBaseInfo.RiskLevel
		}
	}

	appUnlockByIPAddressID, err := i.batchGetAppUnlockByIPAddressID(ipAddressIDsByProxyID)
	if err != nil {
		return nil, err
	}
	for proxyID, ipAddressIDs := range ipAddressIDsByProxyID {
		if resp := result[proxyID]; resp != nil {
			resp.AppUnlock = aggregateAppUnlock(ipAddressIDs, appUnlockByIPAddressID)
		}
	}
	return result, nil
}

func (i ipDetectorImpl) batchGetAppUnlockByIPAddressID(ipAddressIDsByProxyID map[uint][]uint) (map[uint][]*model.IPUnlockInfo, error) {
	allIPIDs := make([]uint, 0)
	seen := make(map[uint]bool)
	for _, ipAddressIDs := range ipAddressIDsByProxyID {
		for _, ipAddressID := range ipAddressIDs {
			if !seen[ipAddressID] {
				seen[ipAddressID] = true
				allIPIDs = append(allIPIDs, ipAddressID)
			}
		}
	}
	if len(allIPIDs) == 0 {
		return map[uint][]*model.IPUnlockInfo{}, nil
	}

	appUnlockList, err := i.IPUnlockInfoRepo.FindByIPAddressIDs(allIPIDs)
	if err != nil {
		log.Errorln("find ip unlock info by ip list failed, err: %v", err)
		return nil, err
	}

	result := make(map[uint][]*model.IPUnlockInfo)
	for _, item := range appUnlockList {
		result[item.IPAddressesID] = append(result[item.IPAddressesID], item)
	}
	return result, nil
}

func aggregateAppUnlock(ipAddressIDs []uint, appUnlockByIPAddressID map[uint][]*model.IPUnlockInfo) []*model.IPUnlockInfo {
	selectedByApp := make(map[string]*model.IPUnlockInfo)
	for _, ipAddressID := range ipAddressIDs {
		for _, item := range appUnlockByIPAddressID[ipAddressID] {
			if item == nil || item.AppName == "" {
				continue
			}
			existing := selectedByApp[item.AppName]
			if existing == nil || appUnlockStatusRank(item.Status) > appUnlockStatusRank(existing.Status) {
				copyItem := *item
				selectedByApp[item.AppName] = &copyItem
			}
		}
	}

	result := make([]*model.IPUnlockInfo, 0, len(selectedByApp))
	for _, item := range selectedByApp {
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].AppName < result[j].AppName
	})
	return result
}

func appUnlockStatusRank(status string) int {
	switch status {
	case "unlock":
		return 4
	case "forbidden":
		return 3
	case "rateLimit":
		return 2
	case "fail":
		return 1
	default:
		return 0
	}
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
