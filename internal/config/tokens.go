package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type TokenConfig struct {
	Limit                int `json:"limit"`
	WindowSeconds        int `json:"window_seconds"`
	BlockDurationSeconds int `json:"block_duration_seconds"`
}

func (t *TokenConfig) GetWindowDuration() time.Duration {
	return time.Duration(t.WindowSeconds) * time.Second
}

func (t *TokenConfig) GetBlockDuration() time.Duration {
	return time.Duration(t.BlockDurationSeconds) * time.Second
}

type TokenConfigs map[string]TokenConfig

// Carrega configurações de tokens a partir de um arquivo JSON
func LoadTokenConfigs(filePath string) (TokenConfigs, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening tokens config file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	var tokenConfigs TokenConfigs
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&tokenConfigs); err != nil {
		return nil, fmt.Errorf("error decoding tokens config: %w", err)
	}

	return tokenConfigs, nil
}

func (tc TokenConfigs) GetTokenConfig(token string) (*TokenConfig, bool) {
	config, exists := tc[token]
	if !exists {
		return nil, false
	}
	return &config, true
}
