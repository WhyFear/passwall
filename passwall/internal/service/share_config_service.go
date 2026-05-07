package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"passwall/internal/model"
	"passwall/internal/repository"
	"strings"
)

type ShareConfigService interface {
	List() ([]*model.ShareConfig, error)
	Create(req ShareConfigRequest) (*model.ShareConfig, error)
	Update(id uint, req ShareConfigRequest) (*model.ShareConfig, error)
	Disable(id uint) error
	Delete(id uint) error
	GetEnabledBySlug(slug string) (*model.ShareConfig, error)
}

type ShareConfigRequest struct {
	Name        string `json:"name"`
	Enabled     *bool  `json:"enabled"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	ProxyType   string `json:"proxy_type"`
	CountryCode string `json:"country_code"`
	RiskLevel   string `json:"risk_level"`
	Sort        string `json:"sort"`
	SortOrder   string `json:"sort_order"`
	Limit       int    `json:"limit"`
	WithIndex   bool   `json:"with_index"`
}

type shareConfigService struct {
	repo repository.ShareConfigRepository
}

func NewShareConfigService(repo repository.ShareConfigRepository) ShareConfigService {
	return &shareConfigService{repo: repo}
}

func (s *shareConfigService) List() ([]*model.ShareConfig, error) {
	return s.repo.FindAll()
}

func (s *shareConfigService) Create(req ShareConfigRequest) (*model.ShareConfig, error) {
	config := buildShareConfig(req)
	config.Enabled = true
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	slug, err := s.generateSlug()
	if err != nil {
		return nil, err
	}
	config.Slug = slug

	if err := validateShareConfig(config); err != nil {
		return nil, err
	}
	if err := s.repo.Create(config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *shareConfigService) Update(id uint, req ShareConfigRequest) (*model.ShareConfig, error) {
	config, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	updated := buildShareConfig(req)
	config.Name = updated.Name
	config.Type = updated.Type
	config.Status = updated.Status
	config.ProxyType = updated.ProxyType
	config.CountryCode = updated.CountryCode
	config.RiskLevel = updated.RiskLevel
	config.Sort = updated.Sort
	config.SortOrder = updated.SortOrder
	config.Limit = updated.Limit
	config.WithIndex = updated.WithIndex
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	if err := validateShareConfig(config); err != nil {
		return nil, err
	}
	if err := s.repo.Update(config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *shareConfigService) Disable(id uint) error {
	return s.repo.Disable(id)
}

func (s *shareConfigService) Delete(id uint) error {
	return s.repo.SoftDelete(id)
}

func (s *shareConfigService) GetEnabledBySlug(slug string) (*model.ShareConfig, error) {
	config, err := s.repo.FindBySlug(slug)
	if err != nil {
		return nil, err
	}
	if !config.Enabled || config.Deleted {
		return nil, errors.New("share config disabled")
	}
	return config, nil
}

func (s *shareConfigService) generateSlug() (string, error) {
	for i := 0; i < 5; i++ {
		bytes := make([]byte, 12)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		slug := base64.RawURLEncoding.EncodeToString(bytes)
		exists, err := s.repo.SlugExists(slug)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
	}
	return "", errors.New("failed to generate unique share slug")
}

func buildShareConfig(req ShareConfigRequest) *model.ShareConfig {
	return &model.ShareConfig{
		Name:        strings.TrimSpace(req.Name),
		Type:        strings.TrimSpace(req.Type),
		Status:      strings.TrimSpace(req.Status),
		ProxyType:   strings.TrimSpace(req.ProxyType),
		CountryCode: strings.TrimSpace(req.CountryCode),
		RiskLevel:   strings.TrimSpace(req.RiskLevel),
		Sort:        strings.TrimSpace(req.Sort),
		SortOrder:   strings.TrimSpace(req.SortOrder),
		Limit:       req.Limit,
		WithIndex:   req.WithIndex,
	}
}

func validateShareConfig(config *model.ShareConfig) error {
	if config.Name == "" {
		return errors.New("name is required")
	}
	if config.Type == "" {
		return errors.New("type is required")
	}
	if config.Limit < 0 {
		return errors.New("limit must be greater than or equal to 0")
	}
	return nil
}
