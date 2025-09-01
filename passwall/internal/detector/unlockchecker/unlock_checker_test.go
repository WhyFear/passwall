package unlockchecker

import (
	"passwall/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecker(t *testing.T) {
	config := `your config here`

	testCheckerList := []struct {
		name        string
		UnlockCheck UnlockCheck
	}{
		{"DisneyPlus", NewDisneyPlusChecker()},
		{"Netflix", NewNetflixUnlockCheck()},
		{"TikTok", NewTikTokUnlockCheck()},
		{"YoutubePremium", NewYoutubePremiumCheck()},
		{"PrimeVideo", NewPrimeVideoUnlockCheck()},
		{"Spotify", NewSpotifyUnlockCheck()},
		{"OpenAI", NewOpenAIUnlockCheck()},
	}

	for _, tc := range testCheckerList {
		t.Run(tc.name, func(t *testing.T) {
			ipProxy := model.NewIPProxy("1.1.1.1", &model.Proxy{
				Config: config,
			})

			resp := tc.UnlockCheck.Check(ipProxy)
			assert.NotNil(t, resp)
		})
	}
}
