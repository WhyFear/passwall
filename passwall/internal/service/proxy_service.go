package service

import (
	"passwall/internal/model"
	"passwall/internal/repository"
)

type ProxyService interface {
	GetProxyByID(id uint) (*model.Proxy, error)
	GetAllProxies(filters map[string]interface{}) ([]*model.Proxy, error)
	GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error)
	CreateProxy(proxy *model.Proxy) error
	BatchCreateProxies(proxies []*model.Proxy) error
	GetTypes() ([]string, error)
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

func (s *DefaultProxyService) GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	// 构建查询参数
	pageQuery := repository.PageQuery{
		Filters: filters,
	}

	// 设置排序
	if sort != "" {
		if sortOrder == "ascend" || sortOrder == "asc" {
			pageQuery.OrderBy = sort + " ASC"
		} else {
			pageQuery.OrderBy = sort + " DESC"
		}
	} else {
		// 默认按下载速度降序排序
		pageQuery.OrderBy = "download_speed DESC"
	}
	// 限制返回的页数
	if page > 0 {
		pageQuery.Page = page
	} else {
		pageQuery.Page = 1
	}

	// 限制返回的代理数量
	if pageSize > 0 {
		pageQuery.PageSize = pageSize
	} else {
		pageQuery.PageSize = 10000
	}

	// 执行查询
	queryResult, err := s.proxyRepo.FindPage(pageQuery)
	if err != nil {
		return nil, 0, err
	}
	return queryResult.Items, queryResult.Total, err
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

func (s *DefaultProxyService) GetTypes() ([]string, error) {
	var types []string
	err := s.proxyRepo.GetTypes(&types)
	return types, err
}
