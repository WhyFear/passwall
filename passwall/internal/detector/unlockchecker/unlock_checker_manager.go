package unlockchecker

import (
	"passwall/internal/detector/model"
	"sync"
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

	// 并发执行所有检测器
	resultChan := make(chan *CheckResult, len(allCheckers))
	var wg sync.WaitGroup

	for _, checker := range allCheckers {
		wg.Add(1)
		go func(ch UnlockCheck) {
			defer wg.Done()
			result := ch.Check(ipProxy)
			resultChan <- result
		}(checker)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(resultChan)

	for i := range results {
		results[i] = <-resultChan
	}

	return results, nil
}
