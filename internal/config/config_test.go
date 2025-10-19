package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	_ = os.Setenv("SERVER_PORT", "9090")
	_ = os.Setenv("RATE_LIMIT_IP", "20")
	_ = os.Setenv("RATE_LIMIT_WINDOW_SECONDS", "2")
	_ = os.Setenv("RATE_LIMIT_BLOCK_DURATION_SECONDS", "600")
	_ = os.Setenv("REDIS_HOST", "test-redis")
	_ = os.Setenv("REDIS_PORT", "6380")
	_ = os.Setenv("REDIS_PASSWORD", "testpass")
	_ = os.Setenv("REDIS_DB", "1")

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.AppEnv)

	assert.Equal(t, 20, cfg.RateLimit.IPLimit)
	assert.Equal(t, 2, cfg.RateLimit.WindowSeconds)
	assert.Equal(t, 600, cfg.RateLimit.BlockDurationSeconds)

	assert.Equal(t, "test-redis", cfg.Redis.Host)
	assert.Equal(t, "6380", cfg.Redis.Port)
	assert.Equal(t, "testpass", cfg.Redis.Password)
	assert.Equal(t, 1, cfg.Redis.DB)

	assert.Equal(t, 2*time.Second, cfg.RateLimit.GetWindowDuration())
	assert.Equal(t, 600*time.Second, cfg.RateLimit.GetBlockDuration())
	assert.Equal(t, "test-redis:6380", cfg.Redis.GetRedisAddr())
}

func TestLoadTokenConfigs(t *testing.T) {
	tokenData := `{
		"test_token": {
			"limit": 100,
			"window_seconds": 1,
			"block_duration_seconds": 300
		},
		"premium_token": {
			"limit": 1000,
			"window_seconds": 1,
			"block_duration_seconds": 60
		}
	}`

	tmpFile, err := os.CreateTemp("", "tokens_test.json")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	_, err = tmpFile.WriteString(tokenData)
	require.NoError(t, err)
	_ = tmpFile.Close()

	tokenConfigs, err := LoadTokenConfigs(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, tokenConfigs)

	testToken, exists := tokenConfigs.GetTokenConfig("test_token")
	require.True(t, exists)
	assert.Equal(t, 100, testToken.Limit)
	assert.Equal(t, 1, testToken.WindowSeconds)
	assert.Equal(t, 300, testToken.BlockDurationSeconds)
	assert.Equal(t, 1*time.Second, testToken.GetWindowDuration())
	assert.Equal(t, 300*time.Second, testToken.GetBlockDuration())

	premiumToken, exists := tokenConfigs.GetTokenConfig("premium_token")
	require.True(t, exists)
	assert.Equal(t, 1000, premiumToken.Limit)
	assert.Equal(t, 1, premiumToken.WindowSeconds)
	assert.Equal(t, 60, premiumToken.BlockDurationSeconds)

	_, exists = tokenConfigs.GetTokenConfig("non_existent")
	assert.False(t, exists)
}

func TestTokenConfigDurationMethods(t *testing.T) {
	tokenConfig := TokenConfig{
		Limit:                50,
		WindowSeconds:        5,
		BlockDurationSeconds: 120,
	}

	assert.Equal(t, 5*time.Second, tokenConfig.GetWindowDuration())
	assert.Equal(t, 120*time.Second, tokenConfig.GetBlockDuration())
}

