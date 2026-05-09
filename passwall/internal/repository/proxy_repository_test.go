package repository

import (
	"testing"

	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProxyRepositoryFindPageFiltersSortsAndPaginates(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "a", Domain: "a.example", Port: 1001, Password: "p1", Type: model.ProxyTypeVMess, Status: model.ProxyStatusOK, DownloadSpeed: 10},
		{Name: "b", Domain: "b.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK, DownloadSpeed: 30},
		{Name: "c", Domain: "c.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusFailed, DownloadSpeed: 20},
		{Name: "d", Domain: "d.example", Port: 1004, Password: "p4", Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned, DownloadSpeed: 40},
	}
	require.NoError(t, repo.BatchCreate(proxies))

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 2,
		OrderBy:  "download_speed desc",
		Filters: map[string]interface{}{
			"type": []string{string(model.ProxyTypeSS)},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(2), result.Total)
	require.Len(t, result.Items, 2)
	assert.Equal(t, "b", result.Items[0].Name)
	assert.Equal(t, "c", result.Items[1].Name)
}

func TestProxyRepositoryBatchCreateDeduplicatesByDomainPortPassword(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	err := repo.BatchCreate([]*model.Proxy{
		{Name: "first", Domain: "same.example", Port: 443, Password: "secret", Type: model.ProxyTypeTrojan, Status: model.ProxyStatusOK},
		{Name: "duplicate", Domain: "same.example", Port: 443, Password: "secret", Type: model.ProxyTypeTrojan, Status: model.ProxyStatusOK},
	})

	require.NoError(t, err)
	all, err := repo.FindAll()
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "first", all[0].Name)
}

func TestProxyRepositoryFindPageFiltersByStatusTypeCountryAndRisk(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "us-low", Domain: "a.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "jp-high", Domain: "b.example", Port: 1002, Password: "p2", Type: model.ProxyTypeTrojan, Status: model.ProxyStatusFailed},
		{Name: "us-high", Domain: "c.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
	}
	require.NoError(t, repo.BatchCreate(proxies))
	seedProxyIPInfo(t, db, proxies[0].ID, "203.0.113.1", 4, "US", "low")
	seedProxyIPInfo(t, db, proxies[1].ID, "203.0.113.2", 4, "JP", "high")
	seedProxyIPInfo(t, db, proxies[2].ID, "203.0.113.3", 4, "US", "high")

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 10,
		OrderBy:  "id asc",
		Filters: map[string]interface{}{
			"status":       []string{"1"},
			"type":         []string{"ss"},
			"country_code": []string{"US"},
			"risk_level":   []string{"high"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "us-high", result.Items[0].Name)
}

func TestProxyRepositoryFindPageDeduplicatesIPJoinTotal(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxy := &model.Proxy{Name: "dual-stack", Domain: "dual.example", Port: 443, Password: "p", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK}
	require.NoError(t, repo.Create(proxy))
	seedProxyIPInfo(t, db, proxy.ID, "203.0.113.10", 4, "US", "low")
	seedProxyIPInfo(t, db, proxy.ID, "2001:db8::10", 6, "US", "low")

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 10,
		Filters: map[string]interface{}{
			"country_code": []string{"US"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "dual-stack", result.Items[0].Name)
}

func TestProxyRepositoryFindPageHidesBannedNodesByDefault(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	require.NoError(t, repo.BatchCreate([]*model.Proxy{
		{Name: "active", Domain: "active.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "banned", Domain: "banned.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
	}))

	result, err := repo.FindPage(PageQuery{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "active", result.Items[0].Name)
}

func TestProxyRepositoryFindPageIgnoresInvalidFiltersSafely(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	require.NoError(t, repo.BatchCreate([]*model.Proxy{
		{Name: "active", Domain: "active.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "failed", Domain: "failed.example", Port: 1002, Password: "p2", Type: model.ProxyTypeTrojan, Status: model.ProxyStatusFailed},
		{Name: "banned", Domain: "banned.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
	}))

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 10,
		Filters: map[string]interface{}{
			"status":       "1",
			"type":         7,
			"country_code": "US",
			"risk_level":   nil,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Items, 2)
}

func newProxyRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Proxy{}, &model.IPAddress{}, &model.ProxyIPAddress{}, &model.IPBaseInfo{}))
	return db
}

func seedProxyIPInfo(t *testing.T, db *gorm.DB, proxyID uint, ip string, ipType uint, countryCode string, riskLevel string) {
	t.Helper()

	ipAddress := &model.IPAddress{IP: ip, IPType: ipType}
	require.NoError(t, db.Create(ipAddress).Error)
	require.NoError(t, db.Create(&model.ProxyIPAddress{
		ProxyID:       proxyID,
		IPAddressesID: ipAddress.ID,
		IPType:        ipType,
		Latest:        true,
	}).Error)
	require.NoError(t, db.Create(&model.IPBaseInfo{
		IPAddressesID: ipAddress.ID,
		CountryCode:   countryCode,
		RiskLevel:     riskLevel,
	}).Error)
}
