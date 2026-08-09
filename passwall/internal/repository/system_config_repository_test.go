package repository

import (
	"errors"
	"testing"

	"passwall/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSystemConfigRepositorySetManyRollsBackOnFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SystemConfig{}))
	require.NoError(t, db.Create([]model.SystemConfig{
		{Key: "concurrent", Value: "7"},
		{Key: "proxy", Value: `{"enabled":false}`},
	}).Error)

	writes := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail-second-config-update", func(tx *gorm.DB) {
		writes++
		if writes == 2 {
			tx.AddError(errors.New("write failed"))
		}
	}))

	repo := NewSystemConfigRepository(db)
	err = repo.SetMany(map[string]string{
		"concurrent": "11",
		"proxy":      `{"enabled":true}`,
	})

	require.ErrorContains(t, err, "write failed")
	values, err := repo.GetAll()
	require.NoError(t, err)
	assert.Equal(t, "7", values["concurrent"])
	assert.JSONEq(t, `{"enabled":false}`, values["proxy"])
}
