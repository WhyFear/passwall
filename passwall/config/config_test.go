package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigRequiresToken(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("PASSWALL_TOKEN", "")

	cfg, err := LoadConfig()

	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PASSWALL_TOKEN")
}

func TestLoadConfigAppliesDefaultsAndEnvironmentSecrets(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("PASSWALL_TOKEN", "secret-token")
	t.Setenv("SCAMALYTICS_HOST", "https://risk.example.test")
	t.Setenv("SCAMALYTICS_USER", "user")
	t.Setenv("SCAMALYTICS_API_KEY", "key")

	cfg, err := LoadConfig()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "secret-token", cfg.Token)
	assert.Equal(t, "127.0.0.1:8080", cfg.Server.Address)
	assert.Equal(t, "sqlite", cfg.Database.Driver)
	assert.Equal(t, "https://risk.example.test", cfg.IPCheck.IPInfo.Scamalytics.Host)
	assert.Equal(t, "user", cfg.IPCheck.IPInfo.Scamalytics.User)
	assert.Equal(t, "key", cfg.IPCheck.IPInfo.Scamalytics.APIKey)
}

func TestExampleConfigUsesCurrentSchema(t *testing.T) {
	configPath, err := filepath.Abs(filepath.Join("..", "..", "config.yaml.example"))
	require.NoError(t, err)
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("PASSWALL_TOKEN", "secret-token")
	t.Setenv("SCAMALYTICS_HOST", "")
	t.Setenv("SCAMALYTICS_USER", "")
	t.Setenv("SCAMALYTICS_API_KEY", "")

	cfg, err := LoadConfig()

	require.NoError(t, err)
	require.Len(t, cfg.ClashAPI.Clients, 1)
	assert.Equal(t, "ws://127.0.0.1:9090", cfg.ClashAPI.Clients[0].URL)
	assert.True(t, cfg.DefaultSub.AutoUpdate)
	assert.Equal(t, "0 0 6,18 * * *", cfg.DefaultSub.Interval)
	require.NotEmpty(t, cfg.CronJobs)
	assert.Equal(t, "每12小时测速并自动封禁", cfg.CronJobs[0].Name)
}

func writeTestConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`
concurrent: 3
database:
  driver: sqlite
  dsn: ":memory:"
`), 0600)
	require.NoError(t, err)
	return path
}
