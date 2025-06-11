package proxy

import (
	"github.com/metacubex/mihomo/log"
	"passwall/internal/model"
	"passwall/internal/repository"
)

type BanProxyReq struct {
	ID                     uint
	SuccessRateThreshold   float32 // 成功率阈值，默认为0
	DownloadSpeedThreshold int     // 下载速度阈值，默认为0
	UploadSpeedThreshold   int     // 上传速度阈值，默认为0
	PingThreshold          int     // 延迟阈值，默认为0
	TestTimes              int     // 测速次数，默认为5
}

type ProxyService interface {
	GetProxyByID(id uint) (*model.Proxy, error)
	GetProxyNumBySubscriptionID(subsId uint) (int64, error)
	GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error)
	CreateProxy(proxy *model.Proxy) error
	BatchCreateProxies(proxies []*model.Proxy) error
	GetTypes() ([]string, error)
	PinProxy(id uint, pin bool) error
	BanProxy(req BanProxyReq) error
}

type DefaultProxyService struct {
	proxyRepo            repository.ProxyRepository
	speedTestHistoryRepo repository.SpeedTestHistoryRepository
}

func NewProxyService(proxyRepo repository.ProxyRepository, speedtestRepo repository.SpeedTestHistoryRepository) ProxyService {
	return &DefaultProxyService{
		proxyRepo:            proxyRepo,
		speedTestHistoryRepo: speedtestRepo,
	}
}

func (s *DefaultProxyService) GetProxyByID(id uint) (*model.Proxy, error) {
	return s.proxyRepo.FindByID(id)
}

func (s *DefaultProxyService) GetProxyNumBySubscriptionID(subsId uint) (int64, error) {
	return s.proxyRepo.CountBySubscriptionID(subsId)
}

func (s *DefaultProxyService) GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	// 构建查询参数
	pageQuery := repository.PageQuery{
		Filters: filters,
	}

	// 设置排序
	pageQuery.OrderBy = "pinned desc,"
	if sort != "" {
		if sortOrder == "ascend" || sortOrder == "asc" {
			pageQuery.OrderBy += sort + " ASC"
		} else {
			pageQuery.OrderBy += sort + " DESC"
		}
	} else {
		// 默认按下载速度降序排序
		pageQuery.OrderBy += "download_speed DESC"
	}
	// 限制返回的页数
	if page > 0 {
		pageQuery.Page = page
	} else {
		pageQuery.Page = 1
	}

	// 限制返回的代理数量
	if pageSize > 0 {
		pageQuery.PageSize = pageSize
	} else {
		pageQuery.PageSize = 10000
	}

	// 执行查询
	queryResult, err := s.proxyRepo.FindPage(pageQuery)
	if err != nil {
		return nil, 0, err
	}
	return queryResult.Items, queryResult.Total, err
}

func (s *DefaultProxyService) CreateProxy(proxy *model.Proxy) error {
	err := s.proxyRepo.Create(proxy)
	if err != nil {
		return err
	}
	return nil
}

func (s *DefaultProxyService) BatchCreateProxies(proxies []*model.Proxy) error {
	err := s.proxyRepo.BatchCreate(proxies)
	if err != nil {
		return err
	}
	return nil
}

func (s *DefaultProxyService) GetTypes() ([]string, error) {
	var types []string
	err := s.proxyRepo.GetTypes(&types)
	return types, err
}
func (s *DefaultProxyService) PinProxy(id uint, pin bool) error {
	err := SafeDBOperation(func() error {
		return s.proxyRepo.PinProxy(id, pin)
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *DefaultProxyService) BanProxy(req BanProxyReq) error {
	if req.ID > 0 {
		proxy, err := s.proxyRepo.FindByID(req.ID)
		if err != nil {
			log.Errorln("找不到指定的代理：%v", req.ID)
			return err
		}
		proxy.Status = model.ProxyStatusBanned
		err = SafeDBOperation(func() error {
			return s.proxyRepo.Update(proxy)
		})
		if err != nil {
			log.Errorln("更新代理状态失败：%v", err)
			return err
		}
		return nil
	}
	// 检查阈值是否合法
	if req.SuccessRateThreshold < 0 || req.SuccessRateThreshold > 100 {
		req.SuccessRateThreshold = 0
	}
	if req.TestTimes <= 0 {
		req.TestTimes = 5
	}
	// 先取出所有满足条件的代理
	allProxies, err := s.proxyRepo.FindAll()
	if err != nil {
		log.Errorln("获取所有代理失败：%v", err)
		return err
	}
	log.Infoln("找到 %d 个代理", len(allProxies))
	bannedCount := 0
	// 遍历所有代理，执行测试并更新状态
	for _, proxy := range allProxies {
		page := repository.PageQuery{
			Page:     1,
			PageSize: req.TestTimes,
		}
		speedTestHistory, err := s.speedTestHistoryRepo.FindByProxyID(proxy.ID, page)
		if err != nil {
			log.Warnln("获取代理测速历史失败：%v", err)
			continue
		}
		// 先判断是否有足够的测速历史记录
		if len(speedTestHistory.Items) < req.TestTimes {
			log.Infoln("代理 %d 的测速历史记录不足 %d 条，跳过", proxy.ID, req.TestTimes)
			continue
		}
		// 计算成功率
		successCount := 0
		for _, history := range speedTestHistory.Items {
			if history.DownloadSpeed <= req.DownloadSpeedThreshold {
				continue
			}
			if history.UploadSpeed <= req.UploadSpeedThreshold {
				continue
			}
			if history.Ping <= req.PingThreshold {
				continue
			}
			successCount++
		}
		successRate := float32(successCount) / float32(req.TestTimes) * 100
		if successRate <= req.SuccessRateThreshold {
			log.Infoln("代理 %d 的成功率为 %.2f%%，低于阈值 %.2f%%，将被封禁", proxy.ID, successRate, req.SuccessRateThreshold)
			bannedCount++
			proxy.Status = model.ProxyStatusBanned
			err = SafeDBOperation(func() error {
				return s.proxyRepo.UpdateProxyStatus(proxy)
			})
			if err != nil {
				log.Errorln("更新代理状态失败：%v", err)
				continue
			}
		}
	}
	log.Infoln("处理完成，共封禁 %d 个代理,共计 %d 个代理", bannedCount, len(allProxies))
	return nil
}
