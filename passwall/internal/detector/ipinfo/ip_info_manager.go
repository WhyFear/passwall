package ipinfo

import (
	"context"
	"fmt"
	"passwall/internal/model"

	"sync"

	"github.com/metacubex/mihomo/log"
	"golang.org/x/sync/errgroup"
)

// RiskManager 管理风险检测器
type RiskManager struct {
	factory IPInfoFactory
}

// NewRiskManager 创建风险管理器
func NewRiskManager(factory IPInfoFactory) *RiskManager {
	return &RiskManager{factory: factory}
}

// DetectByAll 调用所有已注册的风险检测器
func (rm *RiskManager) DetectByAll(ctx context.Context, ipProxy *model.IPProxy) ([]*IPInfoResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	allDetectors := rm.factory.GetAllIPInfoDetectors()
	results := make([]*IPInfoResult, 0)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	for _, detector := range allDetectors {
		d := detector
		g.Go(func() error {
			defer func() {
				if err := recover(); err != nil {
					log.Errorln("batch detect proxy ip failed, detector: %v, err: %v", detector, err)
				}
			}()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			result, err := d.Detect(ctx, ipProxy)
			if err != nil {
				return fmt.Errorf("检测器执行失败: %w", err)
			}
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}
