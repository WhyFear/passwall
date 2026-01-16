package config

import (
	"os"

	"github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v3"
)

// Config 应用程序配置
type Config struct {
	Token      string                          `yaml:"token" json:"-"`
	Concurrent int                             `yaml:"concurrent" json:"concurrent"`
	Server     Server                          `yaml:"server" json:"server"`
	Database   Database                        `yaml:"database" json:"database"`
	Proxy      Proxy                           `yaml:"proxy" json:"proxy"`
	IPCheck    IPCheckConfig                   `yaml:"ip_check" json:"ip_check"`
	ClashAPI   ClashAPIConfig                  `yaml:"clash_api" json:"clash_api"`
	CronJobs   []CronJob                       `yaml:"cron_jobs" json:"cron_jobs"`
	DefaultSub DefaultSubscriptionUpdateConfig `yaml:"default_sub" json:"default_sub"`
}
type DefaultSubscriptionUpdateConfig struct {
	AutoUpdate bool   `yaml:"auto_update" json:"auto_update"`
	Interval   string `yaml:"interval" json:"interval"`
	UseProxy   bool   `yaml:"use_proxy" json:"use_proxy"`
}
type IPCheckConfig struct {
	Enable     bool            `yaml:"enable" json:"enable"`
	IPInfo     IPInfoConfig    `yaml:"ip_info" json:"ip_info"`
	AppUnlock  AppUnlockConfig `yaml:"app_unlock" json:"app_unlock"`
	Refresh    bool            `yaml:"refresh" json:"refresh"`
	Concurrent int             `yaml:"concurrent" json:"concurrent"`
}
type IPInfoConfig struct {
	Enable      bool        `yaml:"enable" json:"enable"`
	Scamalytics Scamalytics `yaml:"scamalytics" json:"scamalytics"`
}
type Scamalytics struct {
	User   string `yaml:"user" json:"user"`
	APIKey string `yaml:"api_key" json:"api_key"`
	Host   string `yaml:"host" json:"host"`
}

type AppUnlockConfig struct {
	Enable bool `yaml:"enable" json:"enable"`
}

// Server 服务器配置
type Server struct {
	Address string `yaml:"address" json:"address"`
}

// Database 数据库配置
type Database struct {
	Driver string `yaml:"driver" json:"driver"`
	DSN    string `yaml:"dsn" json:"dsn"`
}

// Proxy 代理配置
type Proxy struct {
	Enabled bool   `yaml:"enabled" json:"enabled"` // 是否启用代理
	URL     string `yaml:"url" json:"url"`         // 代理URL，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080
}

type ClashAPIConfig struct {
	Enable  bool             `yaml:"enable" json:"enable"`
	Clients []ClashAPIClient `yaml:"clients" json:"clients"`
}

type ClashAPIClient struct {
	URL    string `yaml:"url" json:"url"`
	Secret string `yaml:"secret" json:"secret"`
}

type BanProxyConfig struct {
	Enable                 bool    `yaml:"enable" json:"enable"`
	SuccessRateThreshold   float64 `yaml:"success_rate_threshold" json:"success_rate_threshold"`
	DownloadSpeedThreshold int     `yaml:"download_speed_threshold" json:"download_speed_threshold"`
	UploadSpeedThreshold   int     `yaml:"upload_speed_threshold" json:"upload_speed_threshold"`
	PingThreshold          int     `yaml:"ping_threshold" json:"ping_threshold"`
	TestTimes              int     `yaml:"test_times" json:"test_times"`
}

type TestProxyConfig struct {
	Enable     bool   `yaml:"enable" json:"enable"`
	Status     string `yaml:"status" json:"status"`
	Concurrent int    `yaml:"concurrent" json:"concurrent"`
}

// CronJob 定时任务配置
type CronJob struct {
	Name      string          `yaml:"name" json:"name"`
	Schedule  string          `yaml:"schedule" json:"schedule"`
	TestProxy TestProxyConfig `yaml:"test_proxy" json:"test_proxy"`
	AutoBan   BanProxyConfig  `yaml:"auto_ban" json:"auto_ban"`
	IPCheck   IPCheckConfig   `yaml:"ip_check" json:"ip_check"`
	Webhook   []WebhookConfig `yaml:"webhook" json:"webhook"`
}

type WebhookConfig struct {
	Name   string `yaml:"name" json:"name"`
	Method string `yaml:"method" json:"method"`
	URL    string `yaml:"url" json:"url"`
	Header string `yaml:"header" json:"header"`
	Body   string `yaml:"body" json:"body"`
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, error) {
	// 1. 尝试从环境变量获取配置文件路径
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 3. 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// 4. 解析YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 获取token
	token := os.Getenv("PASSWALL_TOKEN")
	if token == "" {
		panic("PASSWALL_TOKEN is not set")
	}
	config.Token = token

	// 5. 验证配置并设置默认值
	if config.Server.Address == "" {
		config.Server.Address = "127.0.0.1:8080"
	}

	// 写入Scamalytics配置
	config.IPCheck.IPInfo.Scamalytics.Host = os.Getenv("SCAMALYTICS_HOST")
	config.IPCheck.IPInfo.Scamalytics.User = os.Getenv("SCAMALYTICS_USER")
	config.IPCheck.IPInfo.Scamalytics.APIKey = os.Getenv("SCAMALYTICS_API_KEY")
	// 如果三者有一个为空，打印错误日志，并将它们清空
	if config.IPCheck.IPInfo.Scamalytics.Host == "" || config.IPCheck.IPInfo.Scamalytics.User == "" || config.IPCheck.IPInfo.Scamalytics.APIKey == "" {
		config.IPCheck.IPInfo.Scamalytics.Host = ""
		config.IPCheck.IPInfo.Scamalytics.User = ""
		config.IPCheck.IPInfo.Scamalytics.APIKey = ""
		log.Errorln("Scamalytics configuration is incomplete. Please set SCAMALYTICS_HOST, SCAMALYTICS_USER, and SCAMALYTICS_API_KEY.")
	}

	return &config, nil
}
