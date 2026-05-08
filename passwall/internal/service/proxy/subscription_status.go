package proxy

import (
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/metacubex/mihomo/log"
)

func markSubscriptionInvalid(repo repository.SubscriptionRepository, subscription *model.Subscription) error {
	subscription.Status = model.SubscriptionStatusInvalid
	if err := repo.UpdateStatus(subscription); err != nil {
		log.Errorln("更新订阅状态失败: %v", err)
		return err
	}
	return nil
}

func markSubscriptionOK(repo repository.SubscriptionRepository, subscription *model.Subscription, content []byte) error {
	subscription.Status = model.SubscriptionStatusOK
	subscription.Content = string(content)
	if err := repo.UpdateStatusAndContent(subscription); err != nil {
		log.Errorln("更新订阅状态失败: %v", err)
		return err
	}
	return nil
}

func logProxySyncResult(subscription *model.Subscription, result *proxySyncResult) {
	if result == nil {
		return
	}
	log.Infoln(
		"订阅[%s]刷新成功，解析出%d个代理，去重后%d个，新增%d个，更新%d个，跳过%d个",
		subscription.URL,
		result.Parsed,
		result.Unique,
		result.Created,
		result.Updated,
		result.Skipped,
	)
}
