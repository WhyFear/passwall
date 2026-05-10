package service

import (
	"fmt"
	"math"
	"passwall/internal/model"
	"passwall/internal/repository"
)

type SpeedTestHistoryService interface {
	GetSpeedTestHistoryByID(id uint) (*model.SpeedTestHistory, error)
	GetSpeedTestHistoryByProxyID(proxyID uint, page *repository.PageQuery) (repository.SpeedTestHistoryPageResult, error)
	BatchGetSpeedTestHistoryByProxyIDList(proxyIDList []uint) (map[uint][]model.SpeedTestHistory, error)
	GetSuccessRatesByProxyIDList(proxyIDList []uint, limit int) (map[uint]float64, error)
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

func (s *DefaultSpeedTestHistoryService) BatchGetSpeedTestHistoryByProxyIDList(proxyIDList []uint) (map[uint][]model.SpeedTestHistory, error) {
	if len(proxyIDList) == 0 {
		return nil, fmt.Errorf("proxyIDList is empty")
	}
	result, err := s.speedtestHistory.BatchFindByProxyIDList(proxyIDList)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DefaultSpeedTestHistoryService) GetSuccessRatesByProxyIDList(proxyIDList []uint, limit int) (map[uint]float64, error) {
	result := make(map[uint]float64)
	if len(proxyIDList) == 0 {
		return result, nil
	}
	if limit <= 0 {
		limit = 5
	}

	summariesByProxyID, err := s.speedtestHistory.BatchFindLatestSummariesByProxyIDList(proxyIDList, limit)
	if err != nil {
		return nil, err
	}

	for proxyID, summaries := range summariesByProxyID {
		if len(summaries) == 0 {
			continue
		}
		successCount := 0
		for _, summary := range summaries {
			if summary.DownloadSpeed > 0 {
				successCount++
			}
		}
		rate := float64(successCount) / float64(len(summaries)) * 100
		result[proxyID] = math.Round(rate*100) / 100
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
