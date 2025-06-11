package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用程序配置
type Config struct {
	Token      string    `yaml:"token"`
	Concurrent int       `yaml:"concurrent"`
	Server     Server    `yaml:"server"`
	Database   Database  `yaml:"database"`
	Proxy      Proxy     `yaml:"proxy"`
	CronJobs   []CronJob `yaml:"cron_jobs"`
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

// CronJob 定时任务配置
type CronJob struct {
	Name                  string `yaml:"name"`
	Schedule              string `yaml:"schedule"`
	ReloadSubscribeConfig bool   `yaml:"reload_subscribe_config"`
	TestAll               bool   `yaml:"test_all"`
	TestNew               bool   `yaml:"test_new"`
	TestFailed            bool   `yaml:"test_failed"`
	TestSpeed             bool   `yaml:"test_speed"`
	Concurrent            int    `yaml:"concurrent"`
	AutoBan               bool   `yaml:"auto_ban"` // 没处理好，先不用这个参数
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, error) {
	// 1. 尝试从环境变量获取配置文件路径
	configPath := os.Getenv("CONFIG_PATH")

	// 2. 如果环境变量未设置，尝试多个可能的路径
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

	// 5. 验证配置并设置默认值
	if config.Concurrent == 0 {
		config.Concurrent = 5
	}

	if config.Server.Address == "" {
		config.Server.Address = "127.0.0.1:8080"
	}

	return &config, nil
}
