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

func newProxyRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Proxy{}))
	return db
}
