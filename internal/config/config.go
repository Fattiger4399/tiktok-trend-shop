package config

import "os"

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	StaticDir   string
	StorageRoot string
}

func FromEnv() Config {
	return Config{
		DatabaseURL: getEnv("TTS_DATABASE_URL", "sqlite:///./data/go-dev.sqlite3"),
		HTTPAddr:    getEnv("TTS_HTTP_ADDR", ":8080"),
		StaticDir:   getEnv("TTS_STATIC_DIR", ""),
		StorageRoot: getEnv("TTS_STORAGE_ROOT", "./assets"),
	}
}

func getEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
