package proxy

import (
	"testing"

	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyServiceGetProxiesByFiltersBuildsRepositoryQuery(t *testing.T) {
	repo := &capturingProxyRepository{
		result: &repository.PageResult{
			Total: 1,
			Items: []*model.Proxy{{Name: "fast"}},
		},
	}
	service := NewProxyService(repo, nil, task.NewTaskManager())
	filters := map[string]interface{}{"type": []string{"ss"}}

	items, total, err := service.GetProxiesByFilters(filters, "download_speed", "ascend", 2, 25)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "fast", items[0].Name)
	assert.Equal(t, 2, repo.query.Page)
	assert.Equal(t, 25, repo.query.PageSize)
	assert.Equal(t, "pinned desc,download_speed ASC", repo.query.OrderBy)
	assert.Equal(t, filters, repo.query.Filters)
}

func TestProxyServiceGetProxiesByFiltersUsesDefaults(t *testing.T) {
	repo := &capturingProxyRepository{
		result: &repository.PageResult{},
	}
	service := NewProxyService(repo, nil, task.NewTaskManager())

	_, _, err := service.GetProxiesByFilters(nil, "", "", 0, 0)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.query.Page)
	assert.Equal(t, 10000, repo.query.PageSize)
	assert.Equal(t, "pinned desc,download_speed DESC", repo.query.OrderBy)
}

func TestProxyServiceGetProxiesByFiltersRejectsUnknownSortField(t *testing.T) {
	repo := &capturingProxyRepository{
		result: &repository.PageResult{},
	}
	service := NewProxyService(repo, nil, task.NewTaskManager())

	_, _, err := service.GetProxiesByFilters(nil, "name; drop table proxies", "ascend", 1, 10)

	require.NoError(t, err)
	assert.Equal(t, "pinned desc,download_speed DESC", repo.query.OrderBy)
}

type capturingProxyRepository struct {
	repository.ProxyRepository
	query  repository.PageQuery
	result *repository.PageResult
	err    error
}

func (r *capturingProxyRepository) FindPage(query repository.PageQuery) (*repository.PageResult, error) {
	r.query = query
	return r.result, r.err
}
