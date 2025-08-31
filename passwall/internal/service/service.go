package service

import (
	"passwall/config"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"
	"passwall/internal/service/traffic"

	"gorm.io/gorm"

	"passwall/internal/adapter/generator"
	"passwall/internal/adapter/parser"
	"passwall/internal/adapter/speedtester"
	"passwall/internal/repository"
)

// Services 所有服务的集合
type Services struct {
	SubscriptionManager     proxy.SubscriptionManager
	ProxyService            proxy.ProxyService
	SpeedTestHistoryService SpeedTestHistoryService
	ProxyTester             ProxyTester
	NewTester               proxy.Tester
	TaskManager             task.TaskManager
	ParserFactory           parser.ParserFactory
	GeneratorFactory        generator.GeneratorFactory
	SpeedTesterFactory      speedtester.SpeedTesterFactory
	StatisticsService       *traffic.StatisticsService
	IPDetectorService       IPDetectorService
}

// NewServices 初始化所有服务
func NewServices(db *gorm.DB, cfg *config.Config) *Services {
	// 创建仓库集合
	repos := repository.NewRepositories(db)

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
	taskManager := task.NewTaskManager()

	// 创建服务
	subscriptionManager := proxy.NewSubscriptionManager(repos.Subscription, repos.Proxy, parserFactory, taskManager)
	proxyService := proxy.NewProxyService(repos.Proxy, repos.SpeedTestHistory, taskManager, repos.Traffic)
	speedTestHistoryService := NewSpeedTestHistoryService(repos.SpeedTestHistory)

	// 创建代理测试服务
	proxyTester := NewProxyTester(repos.Proxy, repos.Subscription, repos.SpeedTestHistory, speedTesterFactory, parserFactory, taskManager)
	newTester := proxy.NewTester(repos.Proxy, repos.SpeedTestHistory, speedTesterFactory, taskManager)

	statisticsService := traffic.NewTrafficStatisticsService(cfg.ClashAPI.URL, cfg.ClashAPI.Secret, proxyService, repos.Traffic)

	ipDetectorService := NewIPDetector(repos.Proxy, repos.ProxyIPAddress, repos.IPAddress, repos.IPBaseInfo, repos.IPInfo, repos.IPUnlockInfo)

	return &Services{
		SubscriptionManager:     subscriptionManager,
		ProxyService:            proxyService,
		SpeedTestHistoryService: speedTestHistoryService,
		ProxyTester:             proxyTester,
		NewTester:               newTester,
		SpeedTesterFactory:      speedTesterFactory,
		TaskManager:             taskManager,
		ParserFactory:           parserFactory,
		GeneratorFactory:        generatorFactory,
		StatisticsService:       &statisticsService,
		IPDetectorService:       ipDetectorService,
	}
}
