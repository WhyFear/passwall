package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/google/go-cmp/cmp"
	"github.com/metacubex/mihomo/log"
)

type proxySyncer struct {
	parserFactory parser.ParserFactory
	proxyRepo     repository.ProxyRepository
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
	subParser, err := s.parserFactory.GetParser(string(subscription.Type), content)
	if err != nil {
		log.Errorln("获取解析器失败: %v", err)
		return nil, fmt.Errorf("获取解析器失败: %w", err)
	}

	newProxies, err := subParser.Parse(content)
	if err != nil {
		log.Errorln("解析订阅内容失败: %v", err)
		return nil, fmt.Errorf("解析订阅内容失败: %w", err)
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

	if len(toCreate) > 0 {
		if err := s.proxyRepo.BatchCreate(toCreate); err != nil {
			log.Errorln("批量创建代理失败: %v", err)
			return nil, err
		}
		log.Infoln("批量创建了 %d 个新代理", len(toCreate))
	}

	if len(toUpdate) > 0 {
		if err := s.proxyRepo.BatchUpdateProxyConfig(toUpdate); err != nil {
			log.Errorln("批量更新代理配置失败: %v", err)
			return nil, err
		}
		log.Infoln("批量更新了 %d 个代理", len(toUpdate))
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
			log.Errorln("查找旧代理失败: %v", err)
			continue
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
			log.Infoln("跳过重复的代理服务器：%s:%d:%s", proxy.Domain, proxy.Port, proxy.Password)
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
