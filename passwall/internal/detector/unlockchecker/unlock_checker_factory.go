package unlockchecker

import (
	"errors"
	"sync"
)

type unlockCheckFactory struct {
	checkers map[application]UnlockCheck
	mutex    sync.RWMutex
}

func NewUnlockCheckFactory() UnlockCheckFactory {
	return &unlockCheckFactory{
		checkers: make(map[application]UnlockCheck),
	}
}

func (f *unlockCheckFactory) RegisterUnlockChecker(detectorName application, checker UnlockCheck) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.checkers[detectorName] = checker
}

func (f *unlockCheckFactory) GetUnlockChecker(detectorName application) (UnlockCheck, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	if checker, exists := f.checkers[detectorName]; exists {
		return checker, nil
	}
	return nil, errors.New("unlock checker not found: " + string(detectorName))
}

func (f *unlockCheckFactory) GetAllUnlockCheckers() []UnlockCheck {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	allCheckers := make([]UnlockCheck, 0, len(f.checkers))
	for _, checker := range f.checkers {
		allCheckers = append(allCheckers, checker)
	}
	return allCheckers
}
