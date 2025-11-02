// Package config loads environment-driven configuration for the pellets tracker service.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds runtime configuration for the application.
type Config struct {
	DataFile           string
	BackupDir          string
	ListenAddr         string
	TsnetEnabled       bool
	TsnetDir           string
	TsnetHostname      string
	TsnetAuthKey       string
	TsnetListenAddr    string
	BrandImageMaxBytes int64
	RunUID             *int
	RunGID             *int
}

const (
	defaultDataFile           = "data/pellets.json"
	defaultBackupDir          = "data/backups"
	defaultListenAddr         = "127.0.0.1:8080"
	defaultTsnetDir           = "data/tsnet"
	defaultTsnetListen        = ":443"
	defaultBrandImageMaxBytes = 5 * 1024 * 1024
)

// Load builds a Config from environment variables, falling back to defaults
// when values are not provided.
func Load() (*Config, error) {
	runUID, err := getEnvInt("PELLETS_RUN_UID")
	if err != nil {
		return nil, err
	}
	runGID, err := getEnvInt("PELLETS_RUN_GID")
	if err != nil {
		return nil, err
	}
	if (runUID == nil) != (runGID == nil) {
		return nil, fmt.Errorf("PELLETS_RUN_UID and PELLETS_RUN_GID must be set together")
	}

	cfg := &Config{
		DataFile:        getEnv("PELLETS_DATA_FILE", defaultDataFile),
		BackupDir:       getEnv("PELLETS_BACKUP_DIR", defaultBackupDir),
		ListenAddr:      getEnv("PELLETS_LISTEN_ADDR", defaultListenAddr),
		TsnetDir:        getEnv("PELLETS_TSNET_DIR", defaultTsnetDir),
		TsnetHostname:   getEnv("PELLETS_TSNET_HOSTNAME", "pellets"),
		TsnetListenAddr: getEnv("PELLETS_TSNET_LISTEN_ADDR", defaultTsnetListen),
		TsnetAuthKey:    os.Getenv("PELLETS_TSNET_AUTHKEY"),
		RunUID:          runUID,
		RunGID:          runGID,
	}

	brandImageMaxBytes, err := getEnvInt64("PELLETS_BRAND_IMAGE_MAX_BYTES", defaultBrandImageMaxBytes)
	if err != nil {
		return nil, err
	}
	cfg.BrandImageMaxBytes = brandImageMaxBytes

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

	if cfg.RunUID != nil {
		if err := ensureOwnership(cfg); err != nil {
			return nil, err
		}
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

func getEnvInt64(key string, fallback int64) (int64, error) {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid value for %s: %w", key, err)
		}
		if parsed <= 0 {
			return 0, fmt.Errorf("invalid value for %s: must be positive", key)
		}
		return parsed, nil
	}
	return fallback, nil
}

func getEnvInt(key string) (*int, error) {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", key, err)
		}
		if parsed < 0 {
			return nil, fmt.Errorf("invalid value for %s: must be non-negative", key)
		}
		return &parsed, nil
	}
	return nil, nil
}

func ensureOwnership(cfg *Config) error {
	uid := *cfg.RunUID
	gid := *cfg.RunGID

	dataDir := filepath.Dir(cfg.DataFile)
	if err := os.Chown(dataDir, uid, gid); err != nil {
		return fmt.Errorf("chown data dir: %w", err)
	}
	if err := os.Chown(cfg.BackupDir, uid, gid); err != nil {
		return fmt.Errorf("chown backup dir: %w", err)
	}
	if err := chownIfExists(cfg.DataFile, uid, gid); err != nil {
		return err
	}
	if cfg.TsnetEnabled && cfg.TsnetDir != "" {
		if err := os.Chown(cfg.TsnetDir, uid, gid); err != nil {
			return fmt.Errorf("chown tsnet dir: %w", err)
		}
	}
	return nil
}

func chownIfExists(path string, uid, gid int) error {
	if err := os.Chown(path, uid, gid); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("chown %s: %w", path, err)
	}
	return nil
}
