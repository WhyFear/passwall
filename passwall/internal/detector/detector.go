package detector

import (
	"errors"
	"passwall/internal/detector/ipbaseinfo"
	"passwall/internal/detector/ipinfo"
	"passwall/internal/detector/model"
	"passwall/internal/detector/unlockchecker"
	"sync"

	"github.com/metacubex/mihomo/log"
)

type DetectionResult struct {
	BaseInfo     *ipbaseinfo.IPBaseInfo
	IPInfoResult []*ipinfo.IPInfoResult
	UnlockResult []*unlockchecker.CheckResult
}

type DetectorManager struct {
	ipInfoManager      *ipinfo.RiskManager
	unlockCheckManager *unlockchecker.UnlockCheckManager
}

func NewDetectorManager() *DetectorManager {
	ipInfoFactory := ipinfo.NewRiskFactory()
	ipInfoFactory.RegisterIPInfoDetector(ipinfo.DetectorScamalytics, ipinfo.NewScamalyticsRiskDetector())
	ipInfoFactory.RegisterIPInfoDetector(ipinfo.DetectorIPAPI, ipinfo.NewIPAPIRiskDetector())
	ipInfoFactory.RegisterIPInfoDetector(ipinfo.DetectorNodeGet, ipinfo.NewNodeGetRiskDetector())

	unlockFactory := unlockchecker.NewUnlockCheckFactory()
	unlockFactory.RegisterUnlockChecker(unlockchecker.TikTok, unlockchecker.NewTikTokUnlockCheck())
	unlockFactory.RegisterUnlockChecker(unlockchecker.DisneyPlus, unlockchecker.NewDisneyPlusChecker())

	return &DetectorManager{
		ipInfoManager:      ipinfo.NewRiskManager(ipInfoFactory),
		unlockCheckManager: unlockchecker.NewUnlockCheckManager(unlockFactory),
	}
}

func (dm *DetectorManager) DetectAll(ipProxy *model.IPProxy) (*DetectionResult, error) {
	// 第一步：获取基础IP信息（强依赖）
	baseInfo, err := ipbaseinfo.GetProxyIP(ipProxy.ProxyClient)
	if err != nil {
		log.Errorln("DetectAll GetProxyIP error: %v", err)
		return nil, err
	}
	if baseInfo.IPV4 == "" && baseInfo.IPV6 == "" {
		return nil, errors.New("both IPV4 and IPV6 are empty")
	}
	if baseInfo.IPV4 == "" {
		ipProxy.IP = baseInfo.IPV6
	} else {
		ipProxy.IP = baseInfo.IPV4
	}

	// 第二步：并发执行IP信息检测和解锁检测
	var wg sync.WaitGroup
	var ipInfoResult []*ipinfo.IPInfoResult
	var unlockResult []*unlockchecker.CheckResult
	var ipInfoErr, unlockErr error

	wg.Add(2)

	// 并发执行IP信息检测
	go func() {
		defer wg.Done()
		ipInfoResult, ipInfoErr = dm.ipInfoManager.DetectByAll(ipProxy)
	}()

	// 并发执行解锁检测
	go func() {
		defer wg.Done()
		unlockResult, unlockErr = dm.unlockCheckManager.CheckByAll(ipProxy)
	}()

	wg.Wait()

	// 检查并发执行中的错误
	if ipInfoErr != nil {
		log.Errorln("DetectAll IPInfo error: %v", ipInfoErr)
	}
	if unlockErr != nil {
		log.Errorln("DetectAll UnlockCheck error: %v", unlockErr)
	}

	return &DetectionResult{
		BaseInfo:     baseInfo,
		IPInfoResult: ipInfoResult,
		UnlockResult: unlockResult,
	}, nil
}
