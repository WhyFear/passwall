package service

import (
	"testing"

	"passwall/internal/detector"
	"passwall/internal/detector/ipbaseinfo"
	"passwall/internal/detector/ipinfo"
	"passwall/internal/detector/unlockchecker"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPDetectPersisterStoresAddressInfoAndUnlockResults(t *testing.T) {
	ipAddressRepo := &fakeIPAddressRepo{}
	proxyIPRepo := &fakeProxyIPAddressRepo{}
	ipInfoRepo := &fakeIPInfoRepo{}
	ipBaseInfoRepo := &fakeIPBaseInfoRepo{}
	ipUnlockInfoRepo := &fakeIPUnlockInfoRepo{}
	persister := newIPDetectPersister(ipAddressRepo, proxyIPRepo, ipBaseInfoRepo, ipInfoRepo, ipUnlockInfoRepo)

	err := persister.Persist(42, &detector.DetectionResult{
		BaseInfo: &ipbaseinfo.IPBaseInfo{IPV4: "203.0.113.10"},
		IPInfoResultMap: map[string][]*ipinfo.IPInfoResult{
			"203.0.113.10": {
				{
					Detector: ipinfo.DetectorIPAPI,
					Risk:     ipinfo.RiskResult{IPRiskType: ipinfo.IPRiskTypeHigh},
					Geo:      ipinfo.IPGeoInfo{CountryCode: "US"},
					Raw:      "raw-1",
				},
				{
					Detector: ipinfo.DetectorNodeGet,
					Risk:     ipinfo.RiskResult{IPRiskType: ipinfo.IPRiskTypeHigh},
					Geo:      ipinfo.IPGeoInfo{CountryCode: "US"},
					Raw:      "raw-2",
				},
			},
		},
		UnlockResult: []*unlockchecker.CheckResult{
			{APPName: unlockchecker.TikTok, Status: unlockchecker.CheckStatusUnlock, Region: "us"},
		},
	})

	require.NoError(t, err)
	require.Len(t, ipAddressRepo.saved, 1)
	assert.Equal(t, "203.0.113.10", ipAddressRepo.saved[0].IP)
	require.Len(t, proxyIPRepo.saved, 1)
	assert.Equal(t, uint(42), proxyIPRepo.saved[0].ProxyID)
	assert.Equal(t, ipAddressRepo.saved[0].ID, proxyIPRepo.saved[0].IPAddressesID)
	require.Len(t, ipInfoRepo.saved, 2)
	assert.Equal(t, "ipapi", ipInfoRepo.saved[0].Detector)
	require.NotNil(t, ipBaseInfoRepo.saved)
	assert.Equal(t, "high", ipBaseInfoRepo.saved.RiskLevel)
	assert.Equal(t, "US", ipBaseInfoRepo.saved.CountryCode)
	require.Len(t, ipUnlockInfoRepo.saved, 1)
	assert.Equal(t, "TikTok", ipUnlockInfoRepo.saved[0].AppName)
	assert.Equal(t, "unlock", ipUnlockInfoRepo.saved[0].Status)
	assert.Equal(t, "US", ipUnlockInfoRepo.saved[0].Region)
}

type fakeIPAddressRepo struct {
	repository.IPAddressRepository
	nextID uint
	saved  []*model.IPAddress
}

func (r *fakeIPAddressRepo) CreateOrIgnore(ipAddress *model.IPAddress) error {
	r.nextID++
	ipAddress.ID = r.nextID
	r.saved = append(r.saved, ipAddress)
	return nil
}

type fakeProxyIPAddressRepo struct {
	repository.ProxyIPAddressRepository
	saved []*model.ProxyIPAddress
}

func (r *fakeProxyIPAddressRepo) CreateOrUpdate(proxyIPAddress *model.ProxyIPAddress) error {
	r.saved = append(r.saved, proxyIPAddress)
	return nil
}

type fakeIPInfoRepo struct {
	repository.IPInfoRepository
	saved []*model.IPInfo
}

func (r *fakeIPInfoRepo) BatchCreateOrUpdate(ipInfos []*model.IPInfo) error {
	r.saved = append(r.saved, ipInfos...)
	return nil
}

type fakeIPBaseInfoRepo struct {
	repository.IPBaseInfoRepository
	saved *model.IPBaseInfo
}

func (r *fakeIPBaseInfoRepo) CreateOrUpdate(ipBaseInfo *model.IPBaseInfo) error {
	r.saved = ipBaseInfo
	return nil
}

type fakeIPUnlockInfoRepo struct {
	repository.IPUnlockInfoRepository
	saved []*model.IPUnlockInfo
}

func (r *fakeIPUnlockInfoRepo) BatchCreateOrUpdate(ipUnlockInfos []*model.IPUnlockInfo) error {
	r.saved = append(r.saved, ipUnlockInfos...)
	return nil
}
