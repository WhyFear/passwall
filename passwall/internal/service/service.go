package service

import (
	"gorm.io/gorm"

	"passwall/internal/adapter/generator"
	"passwall/internal/adapter/parser"
	"passwall/internal/adapter/speedtester"
	"passwall/internal/repository"
)

// Services 所有服务的集合
type Services struct {
	SubscriptionService     SubscriptionService
	ProxyService            ProxyService
	SpeedTestHistoryService SpeedTestHistoryService
	TaskManager             TaskManager
	ProxyTester             ProxyTester
	ParserFactory           parser.ParserFactory
	GeneratorFactory        generator.GeneratorFactory
	SpeedTesterFactory      speedtester.SpeedTesterFactory
}

// NewServices 初始化所有服务
func NewServices(db *gorm.DB) *Services {
	// 创建仓库
	proxyRepo := repository.NewProxyRepository(db)
	subscriptionRepo := repository.NewSubscriptionRepository(db)
	speedTestHistoryRepo := repository.NewSpeedTestHistoryRepository(db)

	// 创建解析器工厂并注册解析器
	parserFactory := parser.NewParserFactory()
	parserFactory.RegisterParser("share_url", parser.NewShareURLParser())
	parserFactory.RegisterParser("clash", parser.NewClashParser())

	// 创建速度测试器工厂并注册测速器
	speedTesterFactory := speedtester.NewSpeedTesterFactory()
	speedTesterFactory.RegisterSpeedTester(speedtester.NewClashCoreSpeedTester())

	// 创建生成器工厂并注册生成器
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("clash", generator.NewClashGenerator())
	generatorFactory.RegisterGenerator("share_link", generator.NewShareLinkGenerator())

	// 创建任务管理器
	taskManager := NewTaskManager()

	// 创建服务
	subscriptionService := NewSubscriptionService(subscriptionRepo, proxyRepo, parserFactory)
	proxyService := NewProxyService(proxyRepo)
	speedTestHistoryService := NewSpeedTestHistoryService(speedTestHistoryRepo)
	proxyTester := NewProxyTester(proxyRepo, subscriptionRepo, speedTestHistoryRepo, speedTesterFactory, parserFactory, taskManager)

	return &Services{
		SubscriptionService:     subscriptionService,
		ProxyService:            proxyService,
		SpeedTestHistoryService: speedTestHistoryService,
		ProxyTester:             proxyTester,
		SpeedTesterFactory:      speedTesterFactory,
		TaskManager:             taskManager,
		ParserFactory:           parserFactory,
		GeneratorFactory:        generatorFactory,
	}
}
