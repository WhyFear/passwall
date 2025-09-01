package detector

import (
	"errors"
	"passwall/internal/detector/ipbaseinfo"
	"passwall/internal/detector/ipinfo"
	"passwall/internal/detector/unlockchecker"
	"passwall/internal/model"

	"github.com/metacubex/mihomo/log"
	"golang.org/x/sync/errgroup"
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
	unlockFactory.RegisterUnlockChecker(unlockchecker.DisneyPlus, unlockchecker.NewDisneyPlusChecker())
	unlockFactory.RegisterUnlockChecker(unlockchecker.Netflix, unlockchecker.NewNetflixUnlockCheck())
	//unlockFactory.RegisterUnlockChecker(unlockchecker.OpenAI, unlockchecker.NewOpenAIUnlockCheck())
	unlockFactory.RegisterUnlockChecker(unlockchecker.PrimeVideo, unlockchecker.NewPrimeVideoUnlockCheck())
	unlockFactory.RegisterUnlockChecker(unlockchecker.Spotify, unlockchecker.NewSpotifyUnlockCheck())
	unlockFactory.RegisterUnlockChecker(unlockchecker.TikTok, unlockchecker.NewTikTokUnlockCheck())
	unlockFactory.RegisterUnlockChecker(unlockchecker.YouTubePremium, unlockchecker.NewYoutubePremiumCheck())

	return &DetectorManager{
		ipInfoManager:      ipinfo.NewRiskManager(ipInfoFactory),
		unlockCheckManager: unlockchecker.NewUnlockCheckManager(unlockFactory),
	}
}

func (dm *DetectorManager) DetectAll(ipProxy *model.IPProxy, ipInfoEnabled bool, unlockEnable bool) (*DetectionResult, error) {
	if ipProxy == nil {
		return nil, errors.New("ipProxy is nil")
	}
	// 第一步：获取基础IP信息（强依赖）
	baseInfo, err := ipbaseinfo.GetProxyIP(ipProxy.ProxyClient)
	if err != nil {
		log.Errorln("DetectAll GetProxyIP error: %v", err)
		return nil, err
	}
	if baseInfo.IPV4 == "" && baseInfo.IPV6 == "" {
		return nil, errors.New("both IPV4 and IPV6 are empty, maybe the proxy is not working")
	}
	if baseInfo.IPV4 == "" {
		ipProxy.IP = baseInfo.IPV6
	} else {
		ipProxy.IP = baseInfo.IPV4
	}

	// 第二步：并发执行IP信息检测和解锁检测
	var ipInfoResult []*ipinfo.IPInfoResult
	var unlockResult []*unlockchecker.CheckResult

	g := &errgroup.Group{}

	// 并发执行IP信息检测
	if ipInfoEnabled {
		g.Go(func() error {
			result, err := dm.ipInfoManager.DetectByAll(ipProxy)
			if err != nil {
				log.Errorln("DetectAll IPInfo error: %v", err)
				return err
			}
			ipInfoResult = result
			return nil
		})
	}

	// 并发执行解锁检测
	if unlockEnable {
		g.Go(func() error {
			result, err := dm.unlockCheckManager.CheckByAll(ipProxy)
			if err != nil {
				log.Errorln("DetectAll UnlockCheck error: %v", err)
				return err
			}
			unlockResult = result
			return nil
		})
	}

	// 等待所有goroutine完成
	if err := g.Wait(); err != nil {
		log.Errorln("DetectAll error: %v", err)
	}

	return &DetectionResult{
		BaseInfo:     baseInfo,
		IPInfoResult: ipInfoResult,
		UnlockResult: unlockResult,
	}, nil
}
