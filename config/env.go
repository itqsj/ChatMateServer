package config

import "github.com/joho/godotenv"

// LoadEnv 在本地 .env 文件存在时加载环境变量。
func LoadEnv() {
	_ = godotenv.Load()
}
