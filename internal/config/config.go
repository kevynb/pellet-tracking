package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds runtime configuration for the application.
type Config struct {
	DataFile        string
	BackupDir       string
	ListenAddr      string
	TsnetEnabled    bool
	TsnetDir        string
	TsnetHostname   string
	TsnetAuthKey    string
	TsnetListenAddr string
}

const (
	defaultDataFile    = "data/pellets.json"
	defaultBackupDir   = "data/backups"
	defaultListenAddr  = "127.0.0.1:8080"
	defaultTsnetDir    = "data/tsnet"
	defaultTsnetListen = ":443"
)

// Load builds a Config from environment variables, falling back to defaults
// when values are not provided.
func Load() (*Config, error) {
	cfg := &Config{
		DataFile:        getEnv("PELLETS_DATA_FILE", defaultDataFile),
		BackupDir:       getEnv("PELLETS_BACKUP_DIR", defaultBackupDir),
		ListenAddr:      getEnv("PELLETS_LISTEN_ADDR", defaultListenAddr),
		TsnetDir:        getEnv("PELLETS_TSNET_DIR", defaultTsnetDir),
		TsnetHostname:   getEnv("PELLETS_TSNET_HOSTNAME", "pellets"),
		TsnetListenAddr: getEnv("PELLETS_TSNET_LISTEN_ADDR", defaultTsnetListen),
		TsnetAuthKey:    os.Getenv("PELLETS_TSNET_AUTHKEY"),
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
	if cfg.TsnetEnabled && cfg.TsnetDir != "" {
		if err := os.MkdirAll(cfg.TsnetDir, 0o700); err != nil {
			return fmt.Errorf("ensure tsnet dir: %w", err)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
