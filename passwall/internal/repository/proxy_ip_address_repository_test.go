package repository

import (
	"testing"
	"time"

	"passwall/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProxyIPAddressRepositoryCreateOrUpdateRotatesLatest(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyIPAddressRepository(db)
	ipA := createProxyIPAddressTestIP(t, db, "203.0.113.1", 4)
	ipB := createProxyIPAddressTestIP(t, db, "203.0.113.2", 4)

	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{
		ProxyID:       1,
		IPAddressesID: ipA.ID,
		IPType:        4,
	}))
	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{
		ProxyID:       1,
		IPAddressesID: ipB.ID,
		IPType:        4,
	}))

	records := findProxyIPAddressTestRecords(t, db, 1, 4)
	require.Len(t, records, 2)
	assertProxyIPAddressLatestIDs(t, records, []uint{ipB.ID})
}

func TestProxyIPAddressRepositoryCreateOrUpdateRestoresExistingHistoricalIP(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyIPAddressRepository(db)
	ipA := createProxyIPAddressTestIP(t, db, "203.0.113.3", 4)
	ipB := createProxyIPAddressTestIP(t, db, "203.0.113.4", 4)

	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{ProxyID: 1, IPAddressesID: ipA.ID, IPType: 4}))
	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{ProxyID: 1, IPAddressesID: ipB.ID, IPType: 4}))
	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{ProxyID: 1, IPAddressesID: ipA.ID, IPType: 4}))

	records := findProxyIPAddressTestRecords(t, db, 1, 4)
	require.Len(t, records, 2)
	assertProxyIPAddressLatestIDs(t, records, []uint{ipA.ID})
}

func TestProxyIPAddressRepositoryCreateOrUpdateConvergesDirtyLatestRows(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyIPAddressRepository(db)
	ipA := createProxyIPAddressTestIP(t, db, "203.0.113.5", 4)
	ipB := createProxyIPAddressTestIP(t, db, "203.0.113.6", 4)
	ipC := createProxyIPAddressTestIP(t, db, "203.0.113.7", 4)
	ipD := createProxyIPAddressTestIP(t, db, "203.0.113.8", 4)
	for _, ip := range []*model.IPAddress{ipA, ipB, ipC} {
		require.NoError(t, db.Create(&model.ProxyIPAddress{
			ProxyID:       1,
			IPAddressesID: ip.ID,
			IPType:        4,
			Latest:        true,
		}).Error)
	}

	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{
		ProxyID:       1,
		IPAddressesID: ipD.ID,
		IPType:        4,
	}))

	records := findProxyIPAddressTestRecords(t, db, 1, 4)
	require.Len(t, records, 4)
	assertProxyIPAddressLatestIDs(t, records, []uint{ipD.ID})
}

func TestProxyIPAddressRepositoryCreateOrUpdateKeepsIPTypesIndependent(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyIPAddressRepository(db)
	ipv4A := createProxyIPAddressTestIP(t, db, "203.0.113.9", 4)
	ipv4B := createProxyIPAddressTestIP(t, db, "203.0.113.10", 4)
	ipv6 := createProxyIPAddressTestIP(t, db, "2001:db8::1", 6)

	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{ProxyID: 1, IPAddressesID: ipv4A.ID, IPType: 4}))
	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{ProxyID: 1, IPAddressesID: ipv6.ID, IPType: 6}))
	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{ProxyID: 1, IPAddressesID: ipv4B.ID, IPType: 4}))

	assertProxyIPAddressLatestIDs(t, findProxyIPAddressTestRecords(t, db, 1, 4), []uint{ipv4B.ID})
	assertProxyIPAddressLatestIDs(t, findProxyIPAddressTestRecords(t, db, 1, 6), []uint{ipv6.ID})
}

func TestProxyIPAddressRepositoryCreateOrUpdateRefreshesSameIP(t *testing.T) {
	db := newProxyRepositoryTestDB(t)
	repo := NewProxyIPAddressRepository(db)
	ip := createProxyIPAddressTestIP(t, db, "203.0.113.11", 4)
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&model.ProxyIPAddress{
		ProxyID:       1,
		IPAddressesID: ip.ID,
		IPType:        4,
		Latest:        true,
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}).Error)

	require.NoError(t, repo.CreateOrUpdate(&model.ProxyIPAddress{
		ProxyID:       1,
		IPAddressesID: ip.ID,
		IPType:        4,
	}))

	records := findProxyIPAddressTestRecords(t, db, 1, 4)
	require.Len(t, records, 1)
	assertProxyIPAddressLatestIDs(t, records, []uint{ip.ID})
	assert.True(t, records[0].UpdatedAt.After(oldTime))
}

func createProxyIPAddressTestIP(t *testing.T, db *gorm.DB, ip string, ipType uint) *model.IPAddress {
	t.Helper()

	ipAddress := &model.IPAddress{IP: ip, IPType: ipType}
	require.NoError(t, db.Create(ipAddress).Error)
	return ipAddress
}

func findProxyIPAddressTestRecords(t *testing.T, db *gorm.DB, proxyID uint, ipType uint) []*model.ProxyIPAddress {
	t.Helper()

	var records []*model.ProxyIPAddress
	require.NoError(t, db.
		Where("proxy_id = ? AND ip_type = ?", proxyID, ipType).
		Order("id asc").
		Find(&records).Error)
	return records
}

func assertProxyIPAddressLatestIDs(t *testing.T, records []*model.ProxyIPAddress, expected []uint) {
	t.Helper()

	var actual []uint
	for _, record := range records {
		if record.Latest {
			actual = append(actual, record.IPAddressesID)
		}
	}
	assert.ElementsMatch(t, expected, actual)
}
