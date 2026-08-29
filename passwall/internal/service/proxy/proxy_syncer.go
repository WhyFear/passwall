package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/google/go-cmp/cmp"
	"github.com/metacubex/mihomo/log"
)

type proxySyncer struct {
	parserFactory parser.ParserFactory
	proxyRepo     repository.ProxyRepository
	// ponytail: 全局串行可止住跨订阅死锁；同步吞吐成为瓶颈时再改固定锁序或批量 UPSERT。
	syncMu sync.Mutex
}

type proxySyncResult struct {
	Parsed  int
	Unique  int
	Created int
	Updated int
	Skipped int
}

func newProxySyncer(parserFactory parser.ParserFactory, proxyRepo repository.ProxyRepository) *proxySyncer {
	return &proxySyncer{
		parserFactory: parserFactory,
		proxyRepo:     proxyRepo,
	}
}

func (s *proxySyncer) Sync(ctx context.Context, subscription *model.Subscription, content []byte) (*proxySyncResult, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	subParser, err := s.parserFactory.GetParser(string(subscription.Type), content)
	if err != nil {
		log.Errorln("订阅[ID:%d]获取解析器失败，error type: %T", subscription.ID, err)
		return nil, fmt.Errorf("获取解析器失败")
	}

	newProxies, err := subParser.Parse(content)
	if err != nil {
		log.Errorln("订阅[ID:%d]解析失败，error type: %T", subscription.ID, err)
		return nil, fmt.Errorf("解析订阅内容失败")
	}

	if len(newProxies) == 0 {
		log.Errorln("未从订阅中解析出任何代理")
		return nil, fmt.Errorf("未从订阅中解析出任何代理")
	}

	uniqueProxies := dedupeProxies(newProxies)
	toCreate, toUpdate, skipped, err := s.planProxyChanges(ctx, subscription.ID, uniqueProxies)
	if err != nil {
		return nil, err
	}

	changes := append(toCreate, toUpdate...)
	if len(changes) > 0 {
		if err := s.proxyRepo.BatchCreate(changes); err != nil {
			log.Errorln("订阅[ID:%d]批量同步代理失败，error type: %T", subscription.ID, err)
			return nil, fmt.Errorf("批量同步代理失败")
		}
		log.Infoln("批量创建了 %d 个新代理，更新了 %d 个代理", len(toCreate), len(toUpdate))
	}

	return &proxySyncResult{
		Parsed:  len(newProxies),
		Unique:  len(uniqueProxies),
		Created: len(toCreate),
		Updated: len(toUpdate),
		Skipped: skipped,
	}, nil
}

func (s *proxySyncer) planProxyChanges(ctx context.Context, subscriptionID uint, proxies []*model.Proxy) ([]*model.Proxy, []*model.Proxy, int, error) {
	var toCreate []*model.Proxy
	var toUpdate []*model.Proxy
	var skipped int

	for _, newProxy := range proxies {
		select {
		case <-ctx.Done():
			return nil, nil, skipped, ctx.Err()
		default:
		}

		oldProxy, err := s.proxyRepo.FindByDomainPortPassword(newProxy.Domain, newProxy.Port, newProxy.Password)
		if err != nil {
			log.Errorln("订阅[ID:%d]查找代理失败，error type: %T", subscriptionID, err)
			return nil, nil, skipped, fmt.Errorf("查找代理失败")
		}

		if oldProxy == nil {
			newProxy.SubscriptionID = &subscriptionID
			newProxy.Status = model.ProxyStatusPending
			toCreate = append(toCreate, newProxy)
			continue
		}

		if isProxyConfigSame(oldProxy, newProxy) {
			skipped++
			continue
		}

		oldProxy.Name = newProxy.Name
		oldProxy.Type = newProxy.Type
		oldProxy.Config = newProxy.Config
		oldProxy.SubscriptionID = &subscriptionID
		oldProxy.Status = model.ProxyStatusPending
		toUpdate = append(toUpdate, oldProxy)
	}

	return toCreate, toUpdate, skipped, nil
}

func dedupeProxies(proxies []*model.Proxy) []*model.Proxy {
	uniqueProxies := make([]*model.Proxy, 0, len(proxies))
	exist := make(map[string]bool, len(proxies))

	for _, proxy := range proxies {
		key := proxy.DedupKey()
		if exist[key] {
			log.Infoln("跳过重复的代理节点")
			continue
		}
		exist[key] = true
		uniqueProxies = append(uniqueProxies, proxy)
	}

	return uniqueProxies
}

func isProxyConfigSame(oldProxy, newProxy *model.Proxy) bool {
	if oldProxy.Type != newProxy.Type {
		return false
	}

	if oldProxy.Config == newProxy.Config {
		return true
	}

	var oldConfig map[string]interface{}
	if err := json.Unmarshal([]byte(oldProxy.Config), &oldConfig); err != nil {
		return false
	}

	var newConfig map[string]interface{}
	if err := json.Unmarshal([]byte(newProxy.Config), &newConfig); err != nil {
		return false
	}

	delete(oldConfig, "name")
	delete(newConfig, "name")

	return cmp.Equal(oldConfig, newConfig)
}
