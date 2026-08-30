package repository

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestIPBaseInfoRepositoryCreateOrUpdateClearsStringFields(t *testing.T) {
	db := newIPMetadataRepositoryTestDB(t)
	repo := NewIPBaseInfoRepository(db)
	require.NoError(t, db.Create(&model.IPBaseInfo{
		IPAddressesID: 1,
		RiskLevel:     "high",
		CountryCode:   "US",
	}).Error)

	require.NoError(t, repo.CreateOrUpdate(&model.IPBaseInfo{
		IPAddressesID: 1,
		RiskLevel:     "",
		CountryCode:   "",
	}))

	result, err := repo.FindByIPAddressID(1)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.RiskLevel)
	assert.Empty(t, result.CountryCode)
}

func TestIPInfoRepositoryCreateOrUpdateClearsRawAndJSONFields(t *testing.T) {
	db := newIPMetadataRepositoryTestDB(t)
	repo := NewIPInfoRepository(db)
	require.NoError(t, db.Create(&model.IPInfo{
		IPAddressesID: 1,
		Detector:      "ipapi",
		Risk:          datatypes.JSON(`{"IPRiskType":"high"}`),
		Geo:           datatypes.JSON(`{"CountryCode":"US"}`),
		Raw:           "old-raw",
	}).Error)

	require.NoError(t, repo.CreateOrUpdate(&model.IPInfo{
		IPAddressesID: 1,
		Detector:      "ipapi",
		Risk:          datatypes.JSON(`{"IPRiskType":"detect_failed"}`),
		Geo:           datatypes.JSON(`{}`),
		Raw:           "",
	}))

	result, err := repo.FindByIPAddressIDAndDetector(1, "ipapi")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.JSONEq(t, `{"IPRiskType":"detect_failed"}`, string(result.Risk))
	assert.JSONEq(t, `{}`, string(result.Geo))
	assert.Empty(t, result.Raw)
}

func TestIPInfoRepositoryBatchCreateOrUpdateClearsRawAndJSONFields(t *testing.T) {
	db := newIPMetadataRepositoryTestDB(t)
	repo := NewIPInfoRepository(db)
	require.NoError(t, db.Create(&model.IPInfo{
		IPAddressesID: 1,
		Detector:      "scamalytics",
		Risk:          datatypes.JSON(`{"IPRiskType":"low"}`),
		Geo:           datatypes.JSON(`{"CountryCode":"JP"}`),
		Raw:           "old-raw",
	}).Error)

	require.NoError(t, repo.BatchCreateOrUpdate([]*model.IPInfo{{
		IPAddressesID: 1,
		Detector:      "scamalytics",
		Risk:          datatypes.JSON(`{"IPRiskType":"detect_failed"}`),
		Geo:           datatypes.JSON(`{}`),
		Raw:           "",
	}}))

	result, err := repo.FindByIPAddressIDAndDetector(1, "scamalytics")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.JSONEq(t, `{"IPRiskType":"detect_failed"}`, string(result.Risk))
	assert.JSONEq(t, `{}`, string(result.Geo))
	assert.Empty(t, result.Raw)
}

func TestIPUnlockInfoRepositoryCreateOrUpdateClearsRegion(t *testing.T) {
	db := newIPMetadataRepositoryTestDB(t)
	repo := NewIPUnlockInfoRepository(db)
	require.NoError(t, db.Create(&model.IPUnlockInfo{
		IPAddressesID: 1,
		AppName:       "OpenAI",
		Status:        "unlock",
		Region:        "US",
	}).Error)

	require.NoError(t, repo.CreateOrUpdate(&model.IPUnlockInfo{
		IPAddressesID: 1,
		AppName:       "OpenAI",
		Status:        "fail",
		Region:        "",
	}))

	result, err := repo.FindByIPAddressIDAndAppName(1, "OpenAI")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "fail", result.Status)
	assert.Empty(t, result.Region)
}

func TestIPUnlockInfoRepositoryBatchCreateOrUpdateClearsRegion(t *testing.T) {
	db := newIPMetadataRepositoryTestDB(t)
	repo := NewIPUnlockInfoRepository(db)
	require.NoError(t, db.Create(&model.IPUnlockInfo{
		IPAddressesID: 1,
		AppName:       "Netflix",
		Status:        "unlock",
		Region:        "US",
	}).Error)

	require.NoError(t, repo.BatchCreateOrUpdate([]*model.IPUnlockInfo{{
		IPAddressesID: 1,
		AppName:       "Netflix",
		Status:        "fail",
		Region:        "",
	}}))

	result, err := repo.FindByIPAddressIDAndAppName(1, "Netflix")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "fail", result.Status)
	assert.Empty(t, result.Region)
}

func TestIPMetadataRepositoriesConcurrentUpsertKeepsOneBusinessRow(t *testing.T) {
	tests := []struct {
		name  string
		table string
		write func(*gorm.DB, string) error
		count func(*gorm.DB) (int64, error)
	}{
		{
			name:  "base info",
			table: "ip_base_infos",
			write: func(db *gorm.DB, value string) error {
				return NewIPBaseInfoRepository(db).CreateOrUpdate(&model.IPBaseInfo{IPAddressesID: 1, CountryCode: value})
			},
			count: func(db *gorm.DB) (int64, error) {
				var count int64
				err := db.Model(&model.IPBaseInfo{}).Where("ip_addresses_id = ?", 1).Count(&count).Error
				return count, err
			},
		},
		{
			name:  "detector info",
			table: "ip_infos",
			write: func(db *gorm.DB, value string) error {
				return NewIPInfoRepository(db).CreateOrUpdate(&model.IPInfo{IPAddressesID: 1, Detector: "ipapi", Raw: value})
			},
			count: func(db *gorm.DB) (int64, error) {
				var count int64
				err := db.Model(&model.IPInfo{}).Where("ip_addresses_id = ? AND detector = ?", 1, "ipapi").Count(&count).Error
				return count, err
			},
		},
		{
			name:  "unlock info",
			table: "ip_unlock_infos",
			write: func(db *gorm.DB, value string) error {
				return NewIPUnlockInfoRepository(db).CreateOrUpdate(&model.IPUnlockInfo{IPAddressesID: 1, AppName: "OpenAI", Region: value})
			},
			count: func(db *gorm.DB) (int64, error) {
				var count int64
				err := db.Model(&model.IPUnlockInfo{}).Where("ip_addresses_id = ? AND app_name = ?", 1, "OpenAI").Count(&count).Error
				return count, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newIPMetadataRepositoryTestDB(t)
			ready := make(chan struct{}, 2)
			release := make(chan struct{})
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:concurrent-metadata-create", func(tx *gorm.DB) {
				if tx.Statement.Table == tc.table {
					ready <- struct{}{}
					<-release
				}
			}))

			errs := make(chan error, 2)
			var wg sync.WaitGroup
			for _, value := range []string{"first", "second"} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					errs <- tc.write(db, value)
				}()
			}
			for range 2 {
				select {
				case <-ready:
				case <-time.After(time.Second):
					t.Fatal("concurrent writes did not reach create barrier")
				}
			}
			close(release)
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}

			count, err := tc.count(db)
			require.NoError(t, err)
			assert.Equal(t, int64(1), count)
		})
	}
}

func newIPMetadataRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "metadata.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.IPBaseInfo{}, &model.IPInfo{}, &model.IPUnlockInfo{}))
	return db
}
