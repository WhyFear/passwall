package proxy

import (
	"context"
	"fmt"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
	"strconv"

	"github.com/metacubex/mihomo/log"
)

type BanProxyReq struct {
	ID                     uint
	SuccessRateThreshold   float64 // 成功率阈值，默认为0
	DownloadSpeedThreshold int     // 下载速度阈值，默认为0
	UploadSpeedThreshold   int     // 上传速度阈值，默认为0
	PingThreshold          int     // 延迟阈值，默认为0
	TestTimes              int     // 测速次数，默认为5
}

type ProxyService interface {
	GetProxyByID(id uint) (*model.Proxy, error)
	GetProxyNumBySubscriptionID(subsId uint, ignoreBanned bool) (int64, error)
	GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error)
	GetProxyByName(name string) (*model.Proxy, error)
	CreateProxy(proxy *model.Proxy) error
	BatchCreateProxies(proxies []*model.Proxy) error
	GetTypes() ([]string, error)
	PinProxy(id uint, pin bool) error
	BanProxy(ctx context.Context, req BanProxyReq) error
}

type DefaultProxyService struct {
	proxyRepo            repository.ProxyRepository
	speedTestHistoryRepo repository.SpeedTestHistoryRepository
	taskManager          task.TaskManager
}

func NewProxyService(proxyRepo repository.ProxyRepository,
	speedtestRepo repository.SpeedTestHistoryRepository,
	taskManager task.TaskManager) ProxyService {
	return &DefaultProxyService{
		proxyRepo:            proxyRepo,
		speedTestHistoryRepo: speedtestRepo,
		taskManager:          taskManager,
	}
}

func (s *DefaultProxyService) GetProxyByID(id uint) (*model.Proxy, error) {
	return s.proxyRepo.FindByID(id)
}

func (s *DefaultProxyService) GetProxyNumBySubscriptionID(subsId uint, ignoreBanned bool) (int64, error) {
	if ignoreBanned {
		return s.proxyRepo.CountValidBySubscriptionID(subsId)
	}
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

func (s *DefaultProxyService) GetProxyByName(name string) (*model.Proxy, error) {
	return s.proxyRepo.FindByName(name)
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
	return s.proxyRepo.PinProxy(id, pin)
}

func (s *DefaultProxyService) BanProxy(ctx context.Context, req BanProxyReq) error {
	var finishMessage string

	taskCtx, success := s.taskManager.StartTask(ctx, task.TaskTypeBanProxy, 0)
	if !success {
		log.Warnln("已有批量封禁代理任务正在运行")
		return nil
	}

	// 确保在函数返回时完成任务
	defer func() {
		s.taskManager.FinishTask(task.TaskTypeBanProxy, finishMessage)
	}()

	if req.ID > 0 {
		proxy, err := s.proxyRepo.FindByID(req.ID)
		if err != nil {
			errMsg := fmt.Sprintf("找不到指定的代理：%v", req.ID)
			log.Errorln(errMsg)
			finishMessage = errMsg
			return err
		}
		proxy.Status = model.ProxyStatusBanned
		err = s.proxyRepo.UpdateProxyStatus(proxy)
		if err != nil {
			errMsg := fmt.Sprintf("更新代理状态失败：%v", err)
			log.Errorln(errMsg)
			finishMessage = errMsg
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
		finishMessage = "获取所有代理失败：" + err.Error()
		log.Errorln(finishMessage)
		return err
	}

	log.Infoln(fmt.Sprintf("找到 %d 个代理", len(allProxies)))
	s.taskManager.UpdateTotal(task.TaskTypeBanProxy, len(allProxies))
	// 收集需要封禁的代理ID
	proxiesToBan := make([]uint, 0)

	// 遍历所有代理，收集需要封禁的代理
	for i, proxy := range allProxies {
		// 检查任务是否被取消
		select {
		case <-taskCtx.Done():
			log.Warnln("批量封禁代理任务被取消")
			finishMessage = fmt.Sprintf("任务被取消，共封禁 %d 个代理", len(proxiesToBan))
			return nil
		default:
		}

		page := repository.PageQuery{
			Page:     1,
			PageSize: req.TestTimes,
		}
		speedTestHistory, err := s.speedTestHistoryRepo.FindByProxyID(proxy.ID, page)
		if err != nil {
			log.Warnln(fmt.Sprintf("获取代理测速历史失败：%v", err))
			continue
		}
		// 先判断是否有足够的测速历史记录
		if len(speedTestHistory.Items) < req.TestTimes {
			log.Infoln(fmt.Sprintf("代理 %d 的测速历史记录不足 %d 条，跳过", proxy.ID, req.TestTimes))
			continue
		}
		// 计算成功率
		successCount := 0
		for _, history := range speedTestHistory.Items {
			satisfy := false
			if history.DownloadSpeed > req.DownloadSpeedThreshold {
				satisfy = true
			}
			if history.UploadSpeed > req.UploadSpeedThreshold {
				satisfy = true
			}
			if history.Ping > req.PingThreshold {
				satisfy = true
			}
			if satisfy {
				successCount++
			}
		}
		successRate := float64(successCount) / float64(req.TestTimes) * 100
		// 转换为两位小数
		successRate, _ = strconv.ParseFloat(strconv.FormatFloat(successRate, 'f', 2, 64), 64)
		if successRate <= req.SuccessRateThreshold {
			log.Infoln("代理 %d 的成功数为 %v，成功率为 %.2f，低于阈值 %v，将被封禁", proxy.ID, successCount, successRate, req.SuccessRateThreshold)
			proxiesToBan = append(proxiesToBan, proxy.ID)
		}

		// 更新进度
		s.taskManager.UpdateProgress(task.TaskTypeBanProxy, i+1, "")
	}

	// 批量更新需要封禁的代理状态
	if len(proxiesToBan) > 0 {
		if err := s.proxyRepo.BatchUpdateProxyStatus(proxiesToBan, model.ProxyStatusBanned); err != nil {
			log.Errorln("批量更新代理状态失败：%v", err)
		} else {
			log.Infoln("批量封禁了 %d 个代理", len(proxiesToBan))
		}
	}
	log.Infoln("处理完成，共封禁 %d 个代理,共计 %d 个代理", len(proxiesToBan), len(allProxies))

	return nil
}
