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
