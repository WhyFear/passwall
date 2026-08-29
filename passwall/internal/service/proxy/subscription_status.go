package proxy

import (
	"fmt"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/metacubex/mihomo/log"
)

func markSubscriptionInvalid(repo repository.SubscriptionRepository, subscription *model.Subscription) error {
	subscription.Status = model.SubscriptionStatusInvalid
	if err := repo.UpdateStatus(subscription); err != nil {
		log.Errorln("更新订阅[ID:%d]状态失败，error type: %T", subscription.ID, err)
		return fmt.Errorf("更新订阅状态失败")
	}
	return nil
}

func markSubscriptionOK(repo repository.SubscriptionRepository, subscription *model.Subscription, content []byte) error {
	subscription.Status = model.SubscriptionStatusOK
	subscription.Content = string(content)
	if err := repo.UpdateStatusAndContent(subscription); err != nil {
		log.Errorln("更新订阅[ID:%d]状态失败，error type: %T", subscription.ID, err)
		return fmt.Errorf("更新订阅状态失败")
	}
	return nil
}

func logProxySyncResult(subscription *model.Subscription, result *proxySyncResult) {
	if result == nil {
		return
	}
	log.Infoln(
		"订阅[ID:%d]刷新成功，解析出%d个代理，去重后%d个，新增%d个，更新%d个，跳过%d个",
		subscription.ID,
		result.Parsed,
		result.Unique,
		result.Created,
		result.Updated,
		result.Skipped,
	)
}
