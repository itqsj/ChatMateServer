package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	deepSeekAPIKeyEnv  = "DEEPSEEK_API_KEY"
	deepSeekBaseURLEnv = "DEEPSEEK_BASE_URL"
	deepSeekModelEnv   = "DEEPSEEK_MODEL"
)

// DeepSeekConfig 保存从环境变量读取的 DeepSeek 配置。
type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// LoadDeepSeekConfig 从环境变量读取 DeepSeek 配置。
func LoadDeepSeekConfig() (DeepSeekConfig, error) {
	LoadEnv()

	cfg := DeepSeekConfig{
		APIKey:  strings.TrimSpace(os.Getenv(deepSeekAPIKeyEnv)),
		BaseURL: strings.TrimSpace(os.Getenv(deepSeekBaseURLEnv)),
		Model:   strings.TrimSpace(os.Getenv(deepSeekModelEnv)),
	}

	if cfg.APIKey == "" {
		return DeepSeekConfig{}, fmt.Errorf("%s is required", deepSeekAPIKeyEnv)
	}
	if cfg.BaseURL == "" {
		return DeepSeekConfig{}, fmt.Errorf("%s is required", deepSeekBaseURLEnv)
	}
	if cfg.Model == "" {
		return DeepSeekConfig{}, fmt.Errorf("%s is required", deepSeekModelEnv)
	}

	return cfg, nil
}
