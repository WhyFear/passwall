package proxy

import (
	"context"
	"fmt"
	"time"

	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/metacubex/mihomo/log"
)

// SpeedTestService 速度测试服务
type SpeedTestService interface {
	// SaveTestResult 保存测速结果
	SaveTestResult(ctx context.Context, proxy *model.Proxy, result *model.SpeedTestResult) error

	// GetProxyTestHistory 获取代理测速历史
	GetProxyTestHistory(ctx context.Context, proxyID uint, limit int) ([]*model.SpeedTestHistory, error)

	// GetLatestTestResult 获取最新测速结果
	GetLatestTestResult(ctx context.Context, proxyID uint) (*model.SpeedTestHistory, error)
}

// speedTestServiceImpl 速度测试服务实现
type speedTestServiceImpl struct {
	proxyRepo            repository.ProxyRepository
	speedTestHistoryRepo repository.SpeedTestHistoryRepository
}

// NewSpeedTestService 创建速度测试服务
func NewSpeedTestService(
	proxyRepo repository.ProxyRepository,
	speedTestHistoryRepo repository.SpeedTestHistoryRepository,
) SpeedTestService {
	return &speedTestServiceImpl{
		proxyRepo:            proxyRepo,
		speedTestHistoryRepo: speedTestHistoryRepo,
	}
}

// SaveTestResult 保存测速结果
func (s *speedTestServiceImpl) SaveTestResult(ctx context.Context, proxy *model.Proxy, result *model.SpeedTestResult) error {
	if proxy == nil {
		return fmt.Errorf("代理对象不能为空")
	}

	if result == nil {
		return fmt.Errorf("测速结果不能为空")
	}

	// 更新代理状态
	proxy.Ping = result.Ping
	proxy.DownloadSpeed = result.DownloadSpeed
	proxy.UploadSpeed = result.UploadSpeed

	// 设置代理状态
	now := time.Now()
	proxy.LatestTestTime = &now

	if result.Error != nil || result.DownloadSpeed == 0 {
		proxy.Status = model.ProxyStatusFailed
		log.Warnln("代理[%s]测试失败: %v", proxy.Name, result.Error)
	} else {
		proxy.Status = model.ProxyStatusOK
		log.Infoln("代理[%s]测试成功: Ping=%dms, 下载=%s, 上传=%s",
			proxy.Name, result.Ping, formatSpeed(result.DownloadSpeed), formatSpeed(result.UploadSpeed))
	}

	// 保存测速历史记录
	speedTestHistory := &model.SpeedTestHistory{
		ProxyID:       proxy.ID,
		Ping:          result.Ping,
		DownloadSpeed: result.DownloadSpeed,
		UploadSpeed:   result.UploadSpeed,
		TestTime:      now,
		CreatedAt:     now,
	}

	if err := s.speedTestHistoryRepo.Create(speedTestHistory); err != nil {
		return fmt.Errorf("保存测速历史记录失败: %w", err)
	}

	// 更新代理信息
	if err := s.proxyRepo.UpdateSpeedTestInfo(proxy); err != nil {
		return fmt.Errorf("更新代理信息失败: %w", err)
	}

	return nil
}

// GetProxyTestHistory 获取代理测速历史
func (s *speedTestServiceImpl) GetProxyTestHistory(ctx context.Context, proxyID uint, limit int) ([]*model.SpeedTestHistory, error) {
	if limit <= 0 {
		limit = 10 // 默认返回10条记录
	}

	// 创建分页查询参数
	pageQuery := repository.PageQuery{
		Page:     1,
		PageSize: limit,
		OrderBy:  "created_at DESC",
	}

	history, err := s.speedTestHistoryRepo.FindByProxyID(proxyID, pageQuery)
	if err != nil {
		return nil, fmt.Errorf("获取代理测速历史失败: %w", err)
	}

	return history, nil
}

// GetLatestTestResult 获取最新测速结果
func (s *speedTestServiceImpl) GetLatestTestResult(ctx context.Context, proxyID uint) (*model.SpeedTestHistory, error) {
	history, err := s.GetProxyTestHistory(ctx, proxyID, 1)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return nil, nil
	}

	return history[0], nil
}
