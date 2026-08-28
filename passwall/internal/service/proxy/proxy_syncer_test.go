package proxy

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/metacubex/mihomo/log"
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

func TestDedupeProxiesDoesNotLogPassword(t *testing.T) {
	events := log.Subscribe()
	defer log.UnSubscribe(events)
	proxy := &model.Proxy{Domain: "example.test", Port: 443, Password: "password-secret"}

	result := dedupeProxies([]*model.Proxy{proxy, proxy})

	require.Len(t, result, 1)
	select {
	case event := <-events:
		assert.NotContains(t, strings.ToLower(event.Payload), "password-secret")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dedupe log")
	}
}

func TestProxySyncerReturnsParserErrors(t *testing.T) {
	events := log.Subscribe()
	defer log.UnSubscribe(events)
	syncer := newProxySyncer(&fakeParserFactory{err: errors.New("GET https://example.test?token=parser-secret")}, &fakeProxySyncRepository{})

	result, err := syncer.Sync(context.Background(), &model.Subscription{Type: model.SubscriptionTypeClash}, []byte("content"))

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "获取解析器失败")
	assert.NotContains(t, err.Error(), "parser-secret")
	select {
	case event := <-events:
		assert.NotContains(t, event.Payload, "parser-secret")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for parser error log")
	}
}

func TestProxySyncerRejectsEmptyParseResult(t *testing.T) {
	syncer := newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{})

	result, err := syncer.Sync(context.Background(), &model.Subscription{Type: model.SubscriptionTypeClash}, []byte("content"))

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "未从订阅中解析出任何代理")
}

func TestProxySyncerSerializesConcurrentSyncs(t *testing.T) {
	repo := &blockingProxySyncRepository{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	syncer := newProxySyncer(&fakeParserFactory{parser: freshProxyParser{}}, repo)
	errs := make(chan error, 2)

	go func() {
		_, err := syncer.Sync(context.Background(), &model.Subscription{ID: 1, Type: model.SubscriptionTypeClash}, []byte("one"))
		errs <- err
	}()
	<-repo.entered

	go func() {
		_, err := syncer.Sync(context.Background(), &model.Subscription{ID: 2, Type: model.SubscriptionTypeClash}, []byte("two"))
		errs <- err
	}()

	overlapped := false
	select {
	case <-repo.entered:
		overlapped = true
	case <-time.After(50 * time.Millisecond):
	}
	close(repo.release)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	assert.False(t, overlapped)
	assert.Equal(t, int32(1), repo.maxActive.Load())
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

type freshProxyParser struct{}

func (freshProxyParser) Parse([]byte) ([]*model.Proxy, error) {
	return []*model.Proxy{{
		Name: "proxy", Domain: "example.test", Port: 443, Password: "secret", Type: model.ProxyTypeTrojan,
	}}, nil
}

func (freshProxyParser) CanParse([]byte) bool { return true }

func (freshProxyParser) GetType() model.SubscriptionType { return model.SubscriptionTypeClash }

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

type blockingProxySyncRepository struct {
	repository.ProxyRepository
	entered   chan struct{}
	release   chan struct{}
	active    atomic.Int32
	maxActive atomic.Int32
}

func (r *blockingProxySyncRepository) FindByDomainPortPassword(string, int, string) (*model.Proxy, error) {
	return nil, nil
}

func (r *blockingProxySyncRepository) BatchCreate([]*model.Proxy) error {
	active := r.active.Add(1)
	for current := r.maxActive.Load(); active > current && !r.maxActive.CompareAndSwap(current, active); current = r.maxActive.Load() {
	}
	r.entered <- struct{}{}
	<-r.release
	r.active.Add(-1)
	return nil
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
