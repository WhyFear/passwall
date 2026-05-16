package service

import (
	"errors"
	"testing"

	"passwall/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildShareConfigTrimsAndCopiesAppUnlock(t *testing.T) {
	config := buildShareConfig(ShareConfigRequest{
		Name:        " 节点分享 ",
		Type:        " clash ",
		Status:      " 1 ",
		ProxyType:   " trojan ",
		CountryCode: " US ",
		RiskLevel:   " low ",
		AppUnlock:   " Netflix,OpenAI ",
		Sort:        " ping ",
		SortOrder:   " ascend ",
		Limit:       5,
		WithIndex:   true,
	})

	assert.Equal(t, "节点分享", config.Name)
	assert.Equal(t, "clash", config.Type)
	assert.Equal(t, "1", config.Status)
	assert.Equal(t, "trojan", config.ProxyType)
	assert.Equal(t, "US", config.CountryCode)
	assert.Equal(t, "low", config.RiskLevel)
	assert.Equal(t, "Netflix,OpenAI", config.AppUnlock)
	assert.Equal(t, "ping", config.Sort)
	assert.Equal(t, "ascend", config.SortOrder)
	assert.Equal(t, 5, config.Limit)
	assert.True(t, config.WithIndex)
}

func TestValidateShareConfigAllowsAppUnlockFilter(t *testing.T) {
	config := buildShareConfig(ShareConfigRequest{
		Name:      "节点分享",
		Type:      "share_link",
		AppUnlock: "Netflix",
	})

	require.NoError(t, validateShareConfig(config))
}

func TestShareConfigServiceCreatePersistsAppUnlock(t *testing.T) {
	repo := &fakeShareConfigRepo{}
	service := NewShareConfigService(repo)

	enabled := false
	config, err := service.Create(ShareConfigRequest{
		Name:      "节点分享",
		Type:      "share_link",
		AppUnlock: "Netflix,OpenAI",
		Enabled:   &enabled,
	})

	require.NoError(t, err)
	require.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.NotEmpty(t, config.Slug)
	require.NotNil(t, repo.created)
	assert.Equal(t, "Netflix,OpenAI", repo.created.AppUnlock)
}

func TestShareConfigServiceCreateDefaultsEnabled(t *testing.T) {
	repo := &fakeShareConfigRepo{}
	service := NewShareConfigService(repo)

	config, err := service.Create(ShareConfigRequest{
		Name: "节点分享",
		Type: "share_link",
	})

	require.NoError(t, err)
	assert.True(t, config.Enabled)
}

func TestShareConfigServiceCreateValidatesRequest(t *testing.T) {
	service := NewShareConfigService(&fakeShareConfigRepo{})

	config, err := service.Create(ShareConfigRequest{
		Name:  "节点分享",
		Type:  "share_link",
		Limit: -1,
	})

	require.Error(t, err)
	assert.Nil(t, config)
	assert.Equal(t, "limit must be greater than or equal to 0", err.Error())
}

func TestShareConfigServiceUpdatePersistsAppUnlock(t *testing.T) {
	repo := &fakeShareConfigRepo{
		config: &model.ShareConfig{
			ID:        7,
			Name:      "旧配置",
			Type:      "share_link",
			AppUnlock: "Netflix",
			Enabled:   true,
		},
	}
	service := NewShareConfigService(repo)

	enabled := false
	config, err := service.Update(7, ShareConfigRequest{
		Name:      "新配置",
		Type:      "clash",
		AppUnlock: "OpenAI",
		Enabled:   &enabled,
	})

	require.NoError(t, err)
	assert.Equal(t, "新配置", config.Name)
	assert.Equal(t, "clash", config.Type)
	assert.Equal(t, "OpenAI", config.AppUnlock)
	assert.False(t, config.Enabled)
	assert.Same(t, config, repo.updated)
}

func TestShareConfigServiceRejectsDisabledShareConfig(t *testing.T) {
	service := NewShareConfigService(&fakeShareConfigRepo{
		config: &model.ShareConfig{Enabled: false},
	})

	config, err := service.GetEnabledBySlug("slug")

	require.Error(t, err)
	assert.Nil(t, config)
	assert.Equal(t, "share config disabled", err.Error())
}

type fakeShareConfigRepo struct {
	config  *model.ShareConfig
	created *model.ShareConfig
	updated *model.ShareConfig
}

func (f *fakeShareConfigRepo) Create(config *model.ShareConfig) error {
	f.created = config
	return nil
}

func (f *fakeShareConfigRepo) FindByID(id uint) (*model.ShareConfig, error) {
	if f.config == nil || f.config.ID != id {
		return nil, errors.New("not found")
	}
	return f.config, nil
}

func (f *fakeShareConfigRepo) FindBySlug(_ string) (*model.ShareConfig, error) {
	if f.config == nil {
		return nil, errors.New("not found")
	}
	return f.config, nil
}

func (f *fakeShareConfigRepo) FindAll() ([]*model.ShareConfig, error) {
	if f.config == nil {
		return []*model.ShareConfig{}, nil
	}
	return []*model.ShareConfig{f.config}, nil
}

func (f *fakeShareConfigRepo) Update(config *model.ShareConfig) error {
	f.updated = config
	return nil
}

func (f *fakeShareConfigRepo) Disable(_ uint) error {
	return nil
}

func (f *fakeShareConfigRepo) SoftDelete(_ uint) error {
	return nil
}

func (f *fakeShareConfigRepo) SlugExists(_ string) (bool, error) {
	return false, nil
}
