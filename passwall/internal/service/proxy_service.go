package service

import (
	"passwall/internal/model"
	"passwall/internal/repository"
)

type ProxyService interface {
	GetProxyByID(id uint) (*model.Proxy, error)
	GetAllProxies(filters map[string]interface{}) ([]*model.Proxy, error)
	CreateProxy(proxy *model.Proxy) error
	BatchCreateProxies(proxies []*model.Proxy) error
}

type DefaultProxyService struct {
	proxyRepo repository.ProxyRepository
}

func NewProxyService(proxyRepo repository.ProxyRepository) ProxyService {
	return &DefaultProxyService{
		proxyRepo: proxyRepo,
	}
}

func (s *DefaultProxyService) GetProxyByID(id uint) (*model.Proxy, error) {
	return s.proxyRepo.FindByID(id)
}

func (s *DefaultProxyService) GetAllProxies(filters map[string]interface{}) ([]*model.Proxy, error) {
	return s.proxyRepo.FindAll(filters)
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
