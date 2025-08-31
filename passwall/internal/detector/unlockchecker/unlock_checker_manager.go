package unlockchecker

import (
	"passwall/internal/model"

	"sync"

	"golang.org/x/sync/errgroup"
)

// UnlockCheckManager 管理解锁检测器
type UnlockCheckManager struct {
	factory UnlockCheckFactory
}

// NewUnlockCheckManager 创建解锁检测管理器
func NewUnlockCheckManager(factory UnlockCheckFactory) *UnlockCheckManager {
	return &UnlockCheckManager{factory: factory}
}

// CheckByAll 调用所有已注册的解锁检测器
func (m *UnlockCheckManager) CheckByAll(ipProxy *model.IPProxy) ([]*CheckResult, error) {
	allCheckers := m.factory.GetAllUnlockCheckers()
	results := make([]*CheckResult, len(allCheckers))
	var mu sync.Mutex

	g := new(errgroup.Group)
	for i, checker := range allCheckers {
		idx := i
		ch := checker
		g.Go(func() error {
			result := ch.Check(ipProxy)
			mu.Lock()
			results[idx] = result
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}
