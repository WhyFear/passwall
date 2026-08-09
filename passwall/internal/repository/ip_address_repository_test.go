package repository

import (
	"path/filepath"
	"sync"
	"testing"

	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIPAddressRepositoryCreateOrIgnoreReturnsExistingID(t *testing.T) {
	db := newIPAddressRepositoryTestDB(t)
	repo := NewIPAddressRepository(db)
	first := &model.IPAddress{IP: "203.0.113.30", IPType: 4}
	second := &model.IPAddress{IP: "203.0.113.30", IPType: 4}

	require.NoError(t, repo.CreateOrIgnore(first))
	require.NoError(t, repo.CreateOrIgnore(second))

	assert.NotZero(t, first.ID)
	assert.Equal(t, first.ID, second.ID)
	var count int64
	require.NoError(t, db.Model(&model.IPAddress{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestIPAddressRepositoryCreateOrIgnoreIsSafeForConcurrentSameIP(t *testing.T) {
	db := newIPAddressRepositoryTestDB(t)
	repo := NewIPAddressRepository(db)
	const workers = 20
	start := make(chan struct{})
	results := make(chan struct {
		id  uint
		err error
	}, workers)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ipAddress := &model.IPAddress{IP: "203.0.113.31", IPType: 4}
			err := repo.CreateOrIgnore(ipAddress)
			results <- struct {
				id  uint
				err error
			}{ipAddress.ID, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var expectedID uint
	resultCount := 0
	for result := range results {
		resultCount++
		require.NoError(t, result.err)
		require.NotZero(t, result.id)
		if expectedID == 0 {
			expectedID = result.id
		}
		assert.Equal(t, expectedID, result.id)
	}
	assert.Equal(t, workers, resultCount)
	var count int64
	require.NoError(t, db.Model(&model.IPAddress{}).Where("ip = ?", "203.0.113.31").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func newIPAddressRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "ip-address.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.IPAddress{}))
	return db
}
