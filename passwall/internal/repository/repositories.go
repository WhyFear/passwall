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
	IPAddress        IPAddressRepository
	ProxyIPAddress   ProxyIPAddressRepository
	IPInfo           IPInfoRepository
	IPBaseInfo       IPBaseInfoRepository
	IPUnlockInfo     IPUnlockInfoRepository
}

// NewRepositories 创建所有仓库的集合
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Proxy:            NewProxyRepository(db),
		Subscription:     NewSubscriptionRepository(db),
		SpeedTestHistory: NewSpeedTestHistoryRepository(db),
		Traffic:          NewTrafficRepository(db),
		IPAddress:        NewIPAddressRepository(db),
		ProxyIPAddress:   NewProxyIPAddressRepository(db),
		IPInfo:           NewIPInfoRepository(db),
		IPBaseInfo:       NewIPBaseInfoRepository(db),
		IPUnlockInfo:     NewIPUnlockInfoRepository(db),
	}
}
