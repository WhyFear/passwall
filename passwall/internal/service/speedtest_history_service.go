package service

import (
	"passwall/internal/model"
	"passwall/internal/repository"
)

type SpeedTestHistoryService interface {
	GetSpeedTestHistoryByID(id uint) (*model.SpeedTestHistory, error)
	GetSpeedTestHistoryByProxyID(proxyID uint, page *repository.PageQuery) (repository.SpeedTestHistoryPageResult, error)
	SaveSpeedTestHistory(history *model.SpeedTestHistory) (*model.SpeedTestHistory, error)
}

type DefaultSpeedTestHistoryService struct {
	speedtestHistory repository.SpeedTestHistoryRepository
}

func NewSpeedTestHistoryService(speedtestHistory repository.SpeedTestHistoryRepository) SpeedTestHistoryService {
	return &DefaultSpeedTestHistoryService{
		speedtestHistory: speedtestHistory,
	}
}

func (s *DefaultSpeedTestHistoryService) GetSpeedTestHistoryByID(id uint) (*model.SpeedTestHistory, error) {
	speedtestHistory, err := s.speedtestHistory.FindByID(id)
	if err != nil {
		return nil, err
	}
	return speedtestHistory, nil
}

func (s *DefaultSpeedTestHistoryService) GetSpeedTestHistoryByProxyID(proxyID uint, page *repository.PageQuery) (repository.SpeedTestHistoryPageResult, error) {
	result, err := s.speedtestHistory.FindByProxyID(proxyID, *page)
	if err != nil {
		return repository.SpeedTestHistoryPageResult{}, err
	}
	return result, nil
}

func (s *DefaultSpeedTestHistoryService) SaveSpeedTestHistory(history *model.SpeedTestHistory) (*model.SpeedTestHistory, error) {
	err := s.speedtestHistory.Create(history)
	if err != nil {
		return nil, err
	}
	return history, nil
}
