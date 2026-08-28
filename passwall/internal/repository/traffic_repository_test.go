package repository

import (
	"path/filepath"
	"sync"
	"testing"

	"passwall/config"
	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestIncrementTrafficAtomicallyAddsConcurrentBatches(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "traffic.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TrafficStatistics{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	repo := NewTrafficRepository(db)
	require.NoError(t, repo.ValidateProxyIDUnique())

	const writers = 20
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.IncrementTraffic([]model.TrafficStatistics{
				{ProxyID: 1, DownloadTotal: 2, UploadTotal: 1},
				{ProxyID: 2, DownloadTotal: 4, UploadTotal: 3},
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	traffic, err := repo.FindByProxyIDList([]uint{1, 2})
	require.NoError(t, err)
	assert.Equal(t, int64(writers*2), traffic[1].DownloadTotal)
	assert.Equal(t, int64(writers), traffic[1].UploadTotal)
	assert.Equal(t, int64(writers*4), traffic[2].DownloadTotal)
	assert.Equal(t, int64(writers*3), traffic[2].UploadTotal)
}

func TestValidateProxyIDUniqueRejectsDriftedSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "traffic.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE traffic_statistics (id integer primary key, proxy_id integer not null)`).Error)
	require.NoError(t, db.Exec(`CREATE INDEX idx_traffic_proxy_id ON traffic_statistics(proxy_id)`).Error)

	err = NewTrafficRepository(db).ValidateProxyIDUnique()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxy_id")
}

func TestInitDBSQLiteMigratesTrafficStatistics(t *testing.T) {
	db, err := InitDB(config.Database{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "passwall.db")})
	require.NoError(t, err)
	assert.True(t, db.Migrator().HasTable(&model.TrafficStatistics{}))
	require.NoError(t, NewTrafficRepository(db).ValidateProxyIDUnique())
}

func TestIncrementTrafficPostgresSQLQualifiesExistingTotals(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 dbname=passwall"), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	require.NoError(t, err)
	statement := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return (&GormTrafficRepository{db: tx}).incrementTrafficStatement([]model.TrafficStatistics{{ProxyID: 1}})
	})

	assert.Contains(t, statement, "traffic_statistics.download_total + excluded.download_total")
	assert.Contains(t, statement, "traffic_statistics.upload_total + excluded.upload_total")
}
