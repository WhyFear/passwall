package repository

import (
	"gorm.io/gorm"
)

// Repositories 存储所有仓库的集合
type Repositories struct {
	Proxy            ProxyRepository
	Subscription     SubscriptionRepository
	SpeedTestHistory SpeedTestHistoryRepository
	Traffic          TrafficRepository
}

// NewRepositories 创建所有仓库的集合
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Proxy:            NewProxyRepository(db),
		Subscription:     NewSubscriptionRepository(db),
		SpeedTestHistory: NewSpeedTestHistoryRepository(db),
		Traffic:          NewTrafficRepository(db),
	}
}
