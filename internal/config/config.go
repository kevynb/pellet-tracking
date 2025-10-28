package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds runtime configuration for the application.
type Config struct {
	DataFile     string
	BackupDir    string
	ListenAddr   string
	TsnetEnabled bool
}

const (
	defaultDataFile   = "data/pellets.json"
	defaultBackupDir  = "data/backups"
	defaultListenAddr = "127.0.0.1:8080"
)

// Load builds a Config from environment variables, falling back to defaults
// when values are not provided.
func Load() (*Config, error) {
	cfg := &Config{
		DataFile:   getEnv("PELLETS_DATA_FILE", defaultDataFile),
		BackupDir:  getEnv("PELLETS_BACKUP_DIR", defaultBackupDir),
		ListenAddr: getEnv("PELLETS_LISTEN_ADDR", defaultListenAddr),
	}

	if env := os.Getenv("PELLETS_TSNET_ENABLED"); env != "" {
		switch env {
		case "1", "true", "TRUE", "True", "yes", "YES":
			cfg.TsnetEnabled = true
		case "0", "false", "FALSE", "False", "no", "NO":
			cfg.TsnetEnabled = false
		default:
			return nil, fmt.Errorf("invalid value for PELLETS_TSNET_ENABLED: %q", env)
		}
	}

	if err := ensurePaths(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func ensurePaths(cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(cfg.DataFile), 0o755); err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}
	if err := os.MkdirAll(cfg.BackupDir, 0o755); err != nil {
		return fmt.Errorf("ensure backup dir: %w", err)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
