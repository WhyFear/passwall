package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用程序配置
type Config struct {
	Token      string         `yaml:"token"`
	Concurrent int            `yaml:"concurrent"`
	Server     Server         `yaml:"server"`
	Database   Database       `yaml:"database"`
	Proxy      Proxy          `yaml:"proxy"`
	IPCheck    IPCheckConfig  `yaml:"ip_check"`
	ClashAPI   ClashAPIConfig `yaml:"clash_api"`
	CronJobs   []CronJob      `yaml:"cron_jobs"`
}
type IPCheckConfig struct {
	Enable     bool            `yaml:"enable"`
	IPInfo     IPInfoConfig    `yaml:"ip_info"`
	AppUnlock  AppUnlockConfig `yaml:"app_unlock"`
	Refresh    bool            `yaml:"refresh"`
	Concurrent int             `yaml:"concurrent"`
}
type IPInfoConfig struct {
	Enable      bool        `yaml:"enable"`
	Scamalytics Scamalytics `yaml:"scamalytics"`
}
type Scamalytics struct {
	User   string `yaml:"user"`
	APIKey string `yaml:"api_key"`
	Host   string `yaml:"host"`
}

type AppUnlockConfig struct {
	Enable bool `yaml:"enable"`
}

// Server 服务器配置
type Server struct {
	Address string `yaml:"address"`
}

// Database 数据库配置
type Database struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

// Proxy 代理配置
type Proxy struct {
	Enabled bool   `yaml:"enabled"` // 是否启用代理
	URL     string `yaml:"url"`     // 代理URL，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080
}

type ClashAPIConfig struct {
	Enable bool   `yaml:"enable"`
	URL    string `yaml:"url"`
	Secret string `yaml:"secret"`
}

type BanProxyConfig struct {
	Enable                 bool    `yaml:"enable"`
	SuccessRateThreshold   float64 `yaml:"success_rate_threshold"`
	DownloadSpeedThreshold int     `yaml:"download_speed_threshold"`
	UploadSpeedThreshold   int     `yaml:"upload_speed_threshold"`
	PingThreshold          int     `yaml:"ping_threshold"`
	TestTimes              int     `yaml:"test_times"`
}

type TestProxyConfig struct {
	Enable     bool   `yaml:"enable"`
	Status     string `yaml:"status"`
	Concurrent int    `yaml:"concurrent"`
}

// CronJob 定时任务配置
type CronJob struct {
	Name                  string          `yaml:"name"`
	Schedule              string          `yaml:"schedule"`
	ReloadSubscribeConfig bool            `yaml:"reload_subscribe_config"`
	TestProxy             TestProxyConfig `yaml:"test_proxy"`
	AutoBan               BanProxyConfig  `yaml:"auto_ban"`
	IPCheck               IPCheckConfig   `yaml:"ip_check"`
	Webhook               []WebhookConfig `yaml:"webhook"`
}

type WebhookConfig struct {
	Name   string `yaml:"name"`
	Method string `yaml:"method"`
	URL    string `yaml:"url"`
	Header string `yaml:"header"`
	Body   string `yaml:"body"`
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
	if config.Concurrent == 0 {
		config.Concurrent = 5
	}

	if config.Server.Address == "" {
		config.Server.Address = "127.0.0.1:8080"
	}

	// 写入Scamalytics配置
	if config.IPCheck.IPInfo.Scamalytics.Host == "" {
		config.IPCheck.IPInfo.Scamalytics.Host = "https://api11.scamalytics.com/v3/"
	}
	config.IPCheck.IPInfo.Scamalytics.User = os.Getenv("SCAMALYTICS_USER")
	config.IPCheck.IPInfo.Scamalytics.APIKey = os.Getenv("SCAMALYTICS_API_KEY")

	return &config, nil
}
