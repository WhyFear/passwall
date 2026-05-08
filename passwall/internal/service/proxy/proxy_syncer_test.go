package proxy

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxySyncerCreatesUpdatesSkipsAndDeduplicates(t *testing.T) {
	existingSame := &model.Proxy{
		ID:       1,
		Name:     "old same",
		Domain:   "same.example",
		Port:     443,
		Password: "same-secret",
		Type:     model.ProxyTypeTrojan,
		Config:   `{"name":"old same","server":"same.example","port":443,"password":"same-secret"}`,
	}
	existingChanged := &model.Proxy{
		ID:       2,
		Name:     "old changed",
		Domain:   "changed.example",
		Port:     8443,
		Password: "changed-secret",
		Type:     model.ProxyTypeTrojan,
		Config:   `{"name":"old changed","server":"changed.example","port":8443,"password":"old-password"}`,
	}
	proxies := []*model.Proxy{
		{
			Name:     "new",
			Domain:   "new.example",
			Port:     443,
			Password: "new-secret",
			Type:     model.ProxyTypeTrojan,
			Config:   `{"name":"new","server":"new.example","port":443,"password":"new-secret"}`,
		},
		{
			Name:     "new duplicate",
			Domain:   "new.example",
			Port:     443,
			Password: "new-secret",
			Type:     model.ProxyTypeTrojan,
			Config:   `{"name":"new duplicate","server":"new.example","port":443,"password":"new-secret"}`,
		},
		{
			Name:     "same renamed",
			Domain:   "same.example",
			Port:     443,
			Password: "same-secret",
			Type:     model.ProxyTypeTrojan,
			Config:   `{"name":"same renamed","server":"same.example","port":443,"password":"same-secret"}`,
		},
		{
			Name:     "changed",
			Domain:   "changed.example",
			Port:     8443,
			Password: "changed-secret",
			Type:     model.ProxyTypeTrojan,
			Config:   `{"name":"changed","server":"changed.example","port":8443,"password":"new-password"}`,
		},
	}
	repo := &fakeProxySyncRepository{
		existing: map[string]*model.Proxy{
			proxyKey(existingSame):    existingSame,
			proxyKey(existingChanged): existingChanged,
		},
	}
	syncer := newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: proxies}}, repo)
	subscription := &model.Subscription{ID: 99, Type: model.SubscriptionTypeClash}

	result, err := syncer.Sync(context.Background(), subscription, []byte("content"))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 4, result.Parsed)
	assert.Equal(t, 3, result.Unique)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, repo.created, 1)
	assert.Equal(t, uint(99), *repo.created[0].SubscriptionID)
	assert.Equal(t, model.ProxyStatusPending, repo.created[0].Status)
	require.Len(t, repo.updated, 1)
	assert.Equal(t, "changed", repo.updated[0].Name)
	assert.Equal(t, model.ProxyStatusPending, repo.updated[0].Status)
}

func TestProxySyncerReturnsParserErrors(t *testing.T) {
	syncer := newProxySyncer(&fakeParserFactory{err: errors.New("missing parser")}, &fakeProxySyncRepository{})

	result, err := syncer.Sync(context.Background(), &model.Subscription{Type: model.SubscriptionTypeClash}, []byte("content"))

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "获取解析器失败")
}

func TestProxySyncerRejectsEmptyParseResult(t *testing.T) {
	syncer := newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{})

	result, err := syncer.Sync(context.Background(), &model.Subscription{Type: model.SubscriptionTypeClash}, []byte("content"))

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "未从订阅中解析出任何代理")
}

type fakeParserFactory struct {
	parser.ParserFactory
	parser parser.Parser
	err    error
}

func (f *fakeParserFactory) GetParser(typeName string, content []byte) (parser.Parser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.parser, nil
}

type fakeParser struct {
	proxies []*model.Proxy
	err     error
}

func (f *fakeParser) Parse(content []byte) ([]*model.Proxy, error) {
	return f.proxies, f.err
}

func (f *fakeParser) CanParse(content []byte) bool {
	return true
}

func (f *fakeParser) GetType() model.SubscriptionType {
	return model.SubscriptionTypeClash
}

type fakeProxySyncRepository struct {
	repository.ProxyRepository
	existing map[string]*model.Proxy
	created  []*model.Proxy
	updated  []*model.Proxy
}

func (r *fakeProxySyncRepository) FindByDomainPortPassword(domain string, port int, password string) (*model.Proxy, error) {
	if r.existing == nil {
		return nil, nil
	}
	return r.existing[domain+":"+stringPort(port)+":"+password], nil
}

func (r *fakeProxySyncRepository) BatchCreate(proxies []*model.Proxy) error {
	r.created = append(r.created, proxies...)
	return nil
}

func (r *fakeProxySyncRepository) BatchUpdateProxyConfig(proxies []*model.Proxy) error {
	r.updated = append(r.updated, proxies...)
	return nil
}

func proxyKey(proxy *model.Proxy) string {
	return proxy.Domain + ":" + stringPort(proxy.Port) + ":" + proxy.Password
}

func stringPort(port int) string {
	return strconv.Itoa(port)
}
