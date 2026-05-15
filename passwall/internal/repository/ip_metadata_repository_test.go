package repository

import (
	"testing"

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

func newIPMetadataRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.IPBaseInfo{}, &model.IPInfo{}, &model.IPUnlockInfo{}))
	return db
}
