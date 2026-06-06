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
		Filters:  &NodeFilter{Types: []model.ProxyType{model.ProxyTypeSS}},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(2), result.Total)
	require.Len(t, result.Items, 2)
	assert.Equal(t, "b", result.Items[0].Name)
	assert.Equal(t, "c", result.Items[1].Name)
}

func TestProxyRepositoryFindByStatusAndTypesIncludingBanned(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "banned-ss", Domain: "a.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
		{Name: "banned-trojan", Domain: "b.example", Port: 1002, Password: "p2", Type: model.ProxyTypeTrojan, Status: model.ProxyStatusBanned},
		{Name: "ok-ss", Domain: "c.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
	}
	require.NoError(t, repo.BatchCreate(proxies))

	result, err := repo.FindByStatusAndTypesIncludingBanned(
		[]model.ProxyStatus{model.ProxyStatusBanned},
		[]model.ProxyType{model.ProxyTypeSS},
	)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "banned-ss", result[0].Name)
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
		Filters: &NodeFilter{
			Status:      []model.ProxyStatus{model.ProxyStatusOK},
			Types:       []model.ProxyType{model.ProxyTypeSS},
			CountryCode: []string{"US"},
			RiskLevel:   []string{"high"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "us-high", result.Items[0].Name)
}

func TestProxyRepositoryFindPageFiltersCountryAndRiskByLatestIPOnly(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "current-jp-high", Domain: "latest.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "current-us-low", Domain: "match.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
	}
	require.NoError(t, repo.BatchCreate(proxies))

	seedProxyIPInfoWithLatest(t, db, proxies[0].ID, "203.0.113.100", 4, "US", "low", false)
	seedProxyIPInfoWithLatest(t, db, proxies[0].ID, "203.0.113.101", 4, "JP", "high", true)
	seedProxyIPInfoWithLatest(t, db, proxies[1].ID, "203.0.113.102", 4, "US", "low", true)

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 10,
		OrderBy:  "id asc",
		Filters: &NodeFilter{
			CountryCode: []string{"US"},
			RiskLevel:   []string{"low"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "current-us-low", result.Items[0].Name)
}

func TestProxyRepositoryFindPageDeduplicatesIPJoinTotal(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxy := &model.Proxy{Name: "dual-stack", Domain: "dual-count.example", Port: 443, Password: "p", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK}
	require.NoError(t, repo.Create(proxy))
	seedProxyIPInfo(t, db, proxy.ID, "203.0.113.15", 4, "US", "low")
	seedProxyIPInfo(t, db, proxy.ID, "2001:db8::15", 6, "US", "low")

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 1,
		Filters:  &NodeFilter{CountryCode: []string{"US"}},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "dual-stack", result.Items[0].Name)
}

func TestProxyRepositoryFindPageFiltersByUnlockedAppsWithAndSemantics(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "netflix-openai", Domain: "a.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "netflix-only", Domain: "b.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "openai-with-forbidden-netflix", Domain: "c.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "banned", Domain: "d.example", Port: 1004, Password: "p4", Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
	}
	require.NoError(t, repo.BatchCreate(proxies))

	ipv4ID := seedProxyIPUnlock(t, db, proxies[0].ID, "203.0.113.10", 4, true)
	seedUnlockInfo(t, db, ipv4ID, "Netflix", "unlock")
	ipv6ID := seedProxyIPUnlock(t, db, proxies[0].ID, "2001:db8::10", 6, true)
	seedUnlockInfo(t, db, ipv6ID, "OpenAI", "unlock")

	netflixOnlyID := seedProxyIPUnlock(t, db, proxies[1].ID, "203.0.113.11", 4, true)
	seedUnlockInfo(t, db, netflixOnlyID, "Netflix", "unlock")
	oldOpenAIID := seedProxyIPUnlock(t, db, proxies[1].ID, "203.0.113.12", 4, false)
	seedUnlockInfo(t, db, oldOpenAIID, "OpenAI", "unlock")

	forbiddenID := seedProxyIPUnlock(t, db, proxies[2].ID, "203.0.113.13", 4, true)
	seedUnlockInfo(t, db, forbiddenID, "Netflix", "forbidden")
	seedUnlockInfo(t, db, forbiddenID, "OpenAI", "unlock")

	bannedID := seedProxyIPUnlock(t, db, proxies[3].ID, "203.0.113.14", 4, true)
	seedUnlockInfo(t, db, bannedID, "Netflix", "unlock")
	seedUnlockInfo(t, db, bannedID, "OpenAI", "unlock")

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 10,
		OrderBy:  "id asc",
		Filters:  &NodeFilter{AppUnlock: []string{"Netflix", "OpenAI"}},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "netflix-openai", result.Items[0].Name)
}

func TestProxyRepositoryFindPageFiltersByUnlockedAppAndDeduplicatesTotal(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxy := &model.Proxy{Name: "dual-stack", Domain: "dual.example", Port: 443, Password: "p", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK}
	require.NoError(t, repo.Create(proxy))
	ipv4ID := seedProxyIPUnlock(t, db, proxy.ID, "203.0.113.20", 4, true)
	seedUnlockInfo(t, db, ipv4ID, "Netflix", "unlock")
	ipv6ID := seedProxyIPUnlock(t, db, proxy.ID, "2001:db8::20", 6, true)
	seedUnlockInfo(t, db, ipv6ID, "Netflix", "unlock")

	result, err := repo.FindPage(PageQuery{
		Page:     1,
		PageSize: 10,
		Filters:  &NodeFilter{AppUnlock: []string{"Netflix", "Netflix", " "}},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "dual-stack", result.Items[0].Name)
}

func TestProxyRepositoryFindByFilterFiltersByUnlockedAppsWithAndSemantics(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "netflix-openai", Domain: "a.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "netflix-only", Domain: "b.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "openai-with-forbidden-netflix", Domain: "c.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "banned", Domain: "d.example", Port: 1004, Password: "p4", Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
	}
	require.NoError(t, repo.BatchCreate(proxies))

	ipv4ID := seedProxyIPUnlock(t, db, proxies[0].ID, "203.0.113.30", 4, true)
	seedUnlockInfo(t, db, ipv4ID, "Netflix", "unlock")
	ipv6ID := seedProxyIPUnlock(t, db, proxies[0].ID, "2001:db8::30", 6, true)
	seedUnlockInfo(t, db, ipv6ID, "OpenAI", "unlock")

	netflixOnlyID := seedProxyIPUnlock(t, db, proxies[1].ID, "203.0.113.31", 4, true)
	seedUnlockInfo(t, db, netflixOnlyID, "Netflix", "unlock")
	oldOpenAIID := seedProxyIPUnlock(t, db, proxies[1].ID, "203.0.113.32", 4, false)
	seedUnlockInfo(t, db, oldOpenAIID, "OpenAI", "unlock")

	forbiddenID := seedProxyIPUnlock(t, db, proxies[2].ID, "203.0.113.33", 4, true)
	seedUnlockInfo(t, db, forbiddenID, "Netflix", "forbidden")
	seedUnlockInfo(t, db, forbiddenID, "OpenAI", "unlock")

	bannedID := seedProxyIPUnlock(t, db, proxies[3].ID, "203.0.113.34", 4, true)
	seedUnlockInfo(t, db, bannedID, "Netflix", "unlock")
	seedUnlockInfo(t, db, bannedID, "OpenAI", "unlock")

	result, err := repo.FindByFilter(&NodeFilter{
		Types:     []model.ProxyType{model.ProxyTypeSS},
		AppUnlock: []string{"Netflix", "OpenAI"},
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "netflix-openai", result[0].Name)
}

func TestProxyRepositoryFindByFilterFiltersByCountryAndRisk(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "us-low-ss", Domain: "a-filter.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "us-high-ss", Domain: "b-filter.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "jp-low-ss", Domain: "c-filter.example", Port: 1003, Password: "p3", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
	}
	require.NoError(t, repo.BatchCreate(proxies))
	seedProxyIPInfo(t, db, proxies[0].ID, "203.0.113.50", 4, "US", "low")
	seedProxyIPInfo(t, db, proxies[0].ID, "2001:db8::50", 6, "US", "low")
	seedProxyIPInfo(t, db, proxies[1].ID, "203.0.113.51", 4, "US", "high")
	seedProxyIPInfo(t, db, proxies[2].ID, "203.0.113.52", 4, "JP", "low")

	result, err := repo.FindByFilter(&NodeFilter{
		Status:      []model.ProxyStatus{model.ProxyStatusOK},
		Types:       []model.ProxyType{model.ProxyTypeSS},
		CountryCode: []string{"US"},
		RiskLevel:   []string{"low"},
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "us-low-ss", result[0].Name)
}

func TestProxyRepositoryFindByFilterFiltersCountryAndRiskByLatestIPOnly(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxies := []*model.Proxy{
		{Name: "current-jp-high", Domain: "latest-filter.example", Port: 1001, Password: "p1", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
		{Name: "current-us-low", Domain: "match-filter.example", Port: 1002, Password: "p2", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK},
	}
	require.NoError(t, repo.BatchCreate(proxies))

	seedProxyIPInfoWithLatest(t, db, proxies[0].ID, "203.0.113.110", 4, "US", "low", false)
	seedProxyIPInfoWithLatest(t, db, proxies[0].ID, "203.0.113.111", 4, "JP", "high", true)
	seedProxyIPInfoWithLatest(t, db, proxies[1].ID, "203.0.113.112", 4, "US", "low", true)

	result, err := repo.FindByFilter(&NodeFilter{
		CountryCode: []string{"US"},
		RiskLevel:   []string{"low"},
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "current-us-low", result[0].Name)
}

func TestProxyRepositoryFindByFilterNormalizesUnlockedAppFilter(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyRepository(db)

	proxy := &model.Proxy{Name: "dual-stack", Domain: "dual.example", Port: 443, Password: "p", Type: model.ProxyTypeSS, Status: model.ProxyStatusOK}
	require.NoError(t, repo.Create(proxy))
	ipv4ID := seedProxyIPUnlock(t, db, proxy.ID, "203.0.113.40", 4, true)
	seedUnlockInfo(t, db, ipv4ID, "Netflix", "unlock")
	ipv6ID := seedProxyIPUnlock(t, db, proxy.ID, "2001:db8::40", 6, true)
	seedUnlockInfo(t, db, ipv6ID, "Netflix", "unlock")

	result, err := repo.FindByFilter(&NodeFilter{
		AppUnlock: []string{"Netflix", "Netflix", " "},
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "dual-stack", result[0].Name)
}

func TestIPUnlockInfoRepositoryFindByIPAddressIDs(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewIPUnlockInfoRepository(db)

	seedUnlockInfo(t, db, 10, "Netflix", "unlock")
	seedUnlockInfo(t, db, 11, "OpenAI", "fail")
	seedUnlockInfo(t, db, 12, "Claude", "unlock")

	result, err := repo.FindByIPAddressIDs([]uint{10, 11})

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.ElementsMatch(t, []string{"Netflix", "OpenAI"}, []string{result[0].AppName, result[1].AppName})

	empty, err := repo.FindByIPAddressIDs(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
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

	result, err := repo.FindPage(PageQuery{Page: 1, PageSize: 10, Filters: &NodeFilter{CountryCode: []string{" "}}})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Items, 2)
}

func newProxyRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Proxy{}, &model.IPAddress{}, &model.ProxyIPAddress{}, &model.IPBaseInfo{}, &model.IPUnlockInfo{}))
	return db
}

func seedProxyIPInfo(t *testing.T, db *gorm.DB, proxyID uint, ip string, ipType uint, countryCode string, riskLevel string) {
	t.Helper()

	seedProxyIPInfoWithLatest(t, db, proxyID, ip, ipType, countryCode, riskLevel, true)
}

func seedProxyIPInfoWithLatest(t *testing.T, db *gorm.DB, proxyID uint, ip string, ipType uint, countryCode string, riskLevel string, latest bool) {
	t.Helper()

	ipAddress := &model.IPAddress{IP: ip, IPType: ipType}
	require.NoError(t, db.Create(ipAddress).Error)
	proxyIPAddress := &model.ProxyIPAddress{
		ProxyID:       proxyID,
		IPAddressesID: ipAddress.ID,
		IPType:        ipType,
		Latest:        latest,
	}
	require.NoError(t, db.Create(proxyIPAddress).Error)
	if !latest {
		require.NoError(t, db.Model(proxyIPAddress).Update("latest", false).Error)
	}
	require.NoError(t, db.Create(&model.IPBaseInfo{
		IPAddressesID: ipAddress.ID,
		CountryCode:   countryCode,
		RiskLevel:     riskLevel,
	}).Error)
}

func seedProxyIPUnlock(t *testing.T, db *gorm.DB, proxyID uint, ip string, ipType uint, latest bool) uint {
	t.Helper()

	ipAddress := &model.IPAddress{IP: ip, IPType: ipType}
	require.NoError(t, db.Create(ipAddress).Error)
	proxyIPAddress := &model.ProxyIPAddress{
		ProxyID:       proxyID,
		IPAddressesID: ipAddress.ID,
		IPType:        ipType,
		Latest:        latest,
	}
	require.NoError(t, db.Create(proxyIPAddress).Error)
	if !latest {
		require.NoError(t, db.Model(proxyIPAddress).Update("latest", false).Error)
	}
	return ipAddress.ID
}

func seedUnlockInfo(t *testing.T, db *gorm.DB, ipAddressID uint, appName string, status string) {
	t.Helper()

	require.NoError(t, db.Create(&model.IPUnlockInfo{
		IPAddressesID: ipAddressID,
		AppName:       appName,
		Status:        status,
	}).Error)
}
