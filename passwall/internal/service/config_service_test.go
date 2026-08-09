package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"passwall/config"
	"passwall/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigServiceUsesFileDefaultsAndEnvironmentSecrets(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	service := NewConfigService(&fakeSystemConfigRepo{values: map[string]string{}})

	cfg, err := service.GetConfig()

	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Concurrent)
	assert.True(t, cfg.Proxy.Enabled)
	assert.True(t, cfg.IPCheck.Enable)
	assert.True(t, cfg.DefaultSub.AutoUpdate)
	assert.Equal(t, "https://risk.example.test", cfg.IPCheck.IPInfo.Scamalytics.Host)
	assert.Equal(t, "audit-user", cfg.IPCheck.IPInfo.Scamalytics.User)
	assert.Equal(t, "audit-key", cfg.IPCheck.IPInfo.Scamalytics.APIKey)
}

func TestConfigServiceDatabaseOverridesFileButNotEnvironmentSecrets(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	service := NewConfigService(&fakeSystemConfigRepo{values: map[string]string{
		"concurrent": "19",
		"ip_check":   `{"enable":false,"ip_info":{"enable":false,"scamalytics":{"host":"db","user":"db","api_key":"db"}}}`,
	}})

	cfg, err := service.GetConfig()

	require.NoError(t, err)
	assert.Equal(t, 19, cfg.Concurrent)
	assert.False(t, cfg.IPCheck.Enable)
	assert.Equal(t, "https://risk.example.test", cfg.IPCheck.IPInfo.Scamalytics.Host)
	assert.Equal(t, "audit-user", cfg.IPCheck.IPInfo.Scamalytics.User)
	assert.Equal(t, "audit-key", cfg.IPCheck.IPInfo.Scamalytics.APIKey)
}

func TestConfigServiceReturnsRepositoryAndDecodeErrors(t *testing.T) {
	prepareConfigServiceTestEnv(t)

	_, err := NewConfigService(&fakeSystemConfigRepo{getAllErr: errors.New("read failed")}).GetConfig()
	require.ErrorContains(t, err, "read failed")

	_, err = NewConfigService(&fakeSystemConfigRepo{values: map[string]string{"concurrent": "{"}}).GetConfig()
	require.ErrorContains(t, err, "decode system config")
}

func TestConfigServiceUpdateDoesNotReloadAfterPersistenceFailure(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{}, setManyErr: errors.New("write failed")}
	scheduler := &fakeConfigScheduler{}
	statistics := &fakeConfigStatistics{}
	service := NewConfigService(repo)
	service.SetScheduler(scheduler)
	service.SetStatisticsService(statistics)

	err := service.UpdateConfig(map[string]interface{}{
		"concurrent": 11,
		"proxy":      map[string]interface{}{"enabled": true},
	})

	require.ErrorContains(t, err, "write failed")
	assert.False(t, scheduler.called)
	assert.False(t, statistics.stopped)
}

func TestConfigServiceUpdateRejectsWrongTypeBeforePersistence(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{"concurrent": "9"}}
	service := NewConfigService(repo)

	err := service.UpdateConfig(map[string]interface{}{"concurrent": "oops"})

	require.ErrorContains(t, err, `decode system config "concurrent"`)
	assert.Equal(t, "9", repo.values["concurrent"])
	cfg, err := service.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, 9, cfg.Concurrent)
}

func TestConfigServiceUpdateReturnsSchedulerError(t *testing.T) {
	prepareConfigServiceTestEnv(t)
	repo := &fakeSystemConfigRepo{values: map[string]string{}}
	scheduler := &fakeConfigScheduler{err: errors.New("reload failed")}
	statistics := &fakeConfigStatistics{}
	service := NewConfigService(repo)
	service.SetScheduler(scheduler)
	service.SetStatisticsService(statistics)

	err := service.UpdateConfig(map[string]interface{}{"concurrent": 11})

	require.ErrorContains(t, err, "reload failed")
	assert.True(t, scheduler.called)
	assert.False(t, statistics.stopped)
}

func prepareConfigServiceTestEnv(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
concurrent: 7
database:
  driver: sqlite
  dsn: ":memory:"
proxy:
  enabled: true
  url: "http://127.0.0.1:7890"
ip_check:
  enable: true
  ip_info:
    enable: true
  app_unlock:
    enable: true
default_sub:
  auto_update: true
  interval: "0 0 4 * * *"
`), 0600))
	t.Setenv("CONFIG_PATH", path)
	t.Setenv("PASSWALL_TOKEN", "token")
	t.Setenv("SCAMALYTICS_HOST", "https://risk.example.test")
	t.Setenv("SCAMALYTICS_USER", "audit-user")
	t.Setenv("SCAMALYTICS_API_KEY", "audit-key")
}

type fakeSystemConfigRepo struct {
	repository.SystemConfigRepository
	values     map[string]string
	getAllErr  error
	setManyErr error
}

func (r *fakeSystemConfigRepo) GetAll() (map[string]string, error) {
	return r.values, r.getAllErr
}

func (r *fakeSystemConfigRepo) SetMany(values map[string]string) error {
	if r.setManyErr != nil {
		return r.setManyErr
	}
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

type fakeConfigScheduler struct {
	called bool
	err    error
}

func (s *fakeConfigScheduler) Init(config.Config) error {
	s.called = true
	return s.err
}

type fakeConfigStatistics struct {
	stopped bool
}

func (s *fakeConfigStatistics) Start() error { return nil }

func (s *fakeConfigStatistics) Stop() { s.stopped = true }
