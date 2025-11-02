package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envMu prevents concurrent sub-tests from clobbering process-wide environment variables.
var envMu sync.Mutex

func TestLoadBrandImageMaxBytes(t *testing.T) {
	t.Parallel()

	type params struct {
		env string
	}
	type want struct {
		maxBytes  int64
		expectErr bool
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "uses default when unset",
			params: params{env: ""},
			want: want{
				maxBytes: defaultBrandImageMaxBytes,
			},
		},
		{
			name:   "parses custom value",
			params: params{env: "1048576"},
			want: want{
				maxBytes: 1 * 1024 * 1024,
			},
		},
		{
			name:   "rejects invalid value",
			params: params{env: "not-a-number"},
			want: want{
				expectErr: true,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			dataFile := filepath.Join(tempDir, "data.json")
			backupDir := filepath.Join(tempDir, "backups")

			envMu.Lock()
			// Each test case requires a complete config load, so prime the mandatory paths and override the
			// brand image limit in a thread-safe manner.
			origDataFile, dataSet := os.LookupEnv("PELLETS_DATA_FILE")
			require.NoError(t, os.Setenv("PELLETS_DATA_FILE", dataFile), tc.name)
			origBackupDir, backupSet := os.LookupEnv("PELLETS_BACKUP_DIR")
			require.NoError(t, os.Setenv("PELLETS_BACKUP_DIR", backupDir), tc.name)
			origBrand, brandSet := os.LookupEnv("PELLETS_BRAND_IMAGE_MAX_BYTES")
			if tc.params.env == "" {
				require.NoError(t, os.Unsetenv("PELLETS_BRAND_IMAGE_MAX_BYTES"), tc.name)
			} else {
				require.NoError(t, os.Setenv("PELLETS_BRAND_IMAGE_MAX_BYTES", tc.params.env), tc.name)
			}
			t.Cleanup(func() {
				if dataSet {
					require.NoError(t, os.Setenv("PELLETS_DATA_FILE", origDataFile), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_DATA_FILE"), tc.name)
				}
				if backupSet {
					require.NoError(t, os.Setenv("PELLETS_BACKUP_DIR", origBackupDir), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_BACKUP_DIR"), tc.name)
				}
				if brandSet {
					require.NoError(t, os.Setenv("PELLETS_BRAND_IMAGE_MAX_BYTES", origBrand), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_BRAND_IMAGE_MAX_BYTES"), tc.name)
				}
				envMu.Unlock()
			})

			cfg, err := Load()
			if tc.want.expectErr {
				require.Error(t, err, tc.name)
				return
			}

			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.maxBytes, cfg.BrandImageMaxBytes, tc.name)
		})
	}
}

func TestLoadRunIdentityValidation(t *testing.T) {
	t.Parallel()

	type params struct {
		uid string
		gid string
	}
	type want struct {
		expectErr bool
		expectRun bool
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "both unset",
		},
		{
			name: "only uid",
			params: params{
				uid: "1234",
			},
			want: want{expectErr: true},
		},
		{
			name: "only gid",
			params: params{
				gid: "5678",
			},
			want: want{expectErr: true},
		},
		{
			name: "both set",
			params: params{
				uid: "4321",
				gid: "8765",
			},
			want: want{expectRun: true},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			dataFile := filepath.Join(tempDir, "pellets.json")
			backupDir := filepath.Join(tempDir, "backups")

			envMu.Lock()
			origDataFile, dataSet := os.LookupEnv("PELLETS_DATA_FILE")
			require.NoError(t, os.Setenv("PELLETS_DATA_FILE", dataFile), tc.name)
			origBackupDir, backupSet := os.LookupEnv("PELLETS_BACKUP_DIR")
			require.NoError(t, os.Setenv("PELLETS_BACKUP_DIR", backupDir), tc.name)
			origUID, uidSet := os.LookupEnv("PELLETS_RUN_UID")
			if tc.params.uid == "" {
				require.NoError(t, os.Unsetenv("PELLETS_RUN_UID"), tc.name)
			} else {
				require.NoError(t, os.Setenv("PELLETS_RUN_UID", tc.params.uid), tc.name)
			}
			origGID, gidSet := os.LookupEnv("PELLETS_RUN_GID")
			if tc.params.gid == "" {
				require.NoError(t, os.Unsetenv("PELLETS_RUN_GID"), tc.name)
			} else {
				require.NoError(t, os.Setenv("PELLETS_RUN_GID", tc.params.gid), tc.name)
			}
			t.Cleanup(func() {
				if dataSet {
					require.NoError(t, os.Setenv("PELLETS_DATA_FILE", origDataFile), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_DATA_FILE"), tc.name)
				}
				if backupSet {
					require.NoError(t, os.Setenv("PELLETS_BACKUP_DIR", origBackupDir), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_BACKUP_DIR"), tc.name)
				}
				if uidSet {
					require.NoError(t, os.Setenv("PELLETS_RUN_UID", origUID), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_RUN_UID"), tc.name)
				}
				if gidSet {
					require.NoError(t, os.Setenv("PELLETS_RUN_GID", origGID), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_RUN_GID"), tc.name)
				}
				envMu.Unlock()
			})

			cfg, err := Load()
			if tc.want.expectErr {
				require.Error(t, err, tc.name)
				return
			}

			require.NoError(t, err, tc.name)
			if tc.want.expectRun {
				require.NotNil(t, cfg.RunUID, tc.name)
				require.NotNil(t, cfg.RunGID, tc.name)
			} else {
				assert.Nil(t, cfg.RunUID, tc.name)
				assert.Nil(t, cfg.RunGID, tc.name)
			}
		})
	}
}

func TestLoadRunIdentityChown(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("chown not supported on windows")
	}

	type params struct {
		uid int
		gid int
	}
	type want struct {
		uid int
		gid int
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "applies ownership to data and backups",
			params: params{
				uid: 1234,
				gid: 2345,
			},
			want: want{
				uid: 1234,
				gid: 2345,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			dataDir := filepath.Join(tempDir, "data")
			dataFile := filepath.Join(dataDir, "pellets.json")
			backupDir := filepath.Join(tempDir, "backups")

			require.NoError(t, os.MkdirAll(dataDir, 0o755), tc.name)
			require.NoError(t, os.WriteFile(dataFile, []byte("{}"), 0o600), tc.name)

			envMu.Lock()
			origDataFile, dataSet := os.LookupEnv("PELLETS_DATA_FILE")
			require.NoError(t, os.Setenv("PELLETS_DATA_FILE", dataFile), tc.name)
			origBackupDir, backupSet := os.LookupEnv("PELLETS_BACKUP_DIR")
			require.NoError(t, os.Setenv("PELLETS_BACKUP_DIR", backupDir), tc.name)
			origUID, uidSet := os.LookupEnv("PELLETS_RUN_UID")
			require.NoError(t, os.Setenv("PELLETS_RUN_UID", strconv.Itoa(tc.params.uid)), tc.name)
			origGID, gidSet := os.LookupEnv("PELLETS_RUN_GID")
			require.NoError(t, os.Setenv("PELLETS_RUN_GID", strconv.Itoa(tc.params.gid)), tc.name)
			t.Cleanup(func() {
				if dataSet {
					require.NoError(t, os.Setenv("PELLETS_DATA_FILE", origDataFile), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_DATA_FILE"), tc.name)
				}
				if backupSet {
					require.NoError(t, os.Setenv("PELLETS_BACKUP_DIR", origBackupDir), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_BACKUP_DIR"), tc.name)
				}
				if uidSet {
					require.NoError(t, os.Setenv("PELLETS_RUN_UID", origUID), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_RUN_UID"), tc.name)
				}
				if gidSet {
					require.NoError(t, os.Setenv("PELLETS_RUN_GID", origGID), tc.name)
				} else {
					require.NoError(t, os.Unsetenv("PELLETS_RUN_GID"), tc.name)
				}
				envMu.Unlock()
			})

			cfg, err := Load()
			require.NoError(t, err, tc.name)
			require.NotNil(t, cfg.RunUID, tc.name)
			require.NotNil(t, cfg.RunGID, tc.name)

			assert.Equal(t, tc.want.uid, *cfg.RunUID, tc.name)
			assert.Equal(t, tc.want.gid, *cfg.RunGID, tc.name)

			dataInfo, err := os.Stat(dataDir)
			require.NoError(t, err, tc.name)
			backupInfo, err := os.Stat(backupDir)
			require.NoError(t, err, tc.name)
			fileInfo, err := os.Stat(dataFile)
			require.NoError(t, err, tc.name)

			assert.Equal(t, uint32(tc.want.uid), dataInfo.Sys().(*syscall.Stat_t).Uid, tc.name)
			assert.Equal(t, uint32(tc.want.gid), dataInfo.Sys().(*syscall.Stat_t).Gid, tc.name)
			assert.Equal(t, uint32(tc.want.uid), backupInfo.Sys().(*syscall.Stat_t).Uid, tc.name)
			assert.Equal(t, uint32(tc.want.gid), backupInfo.Sys().(*syscall.Stat_t).Gid, tc.name)
			assert.Equal(t, uint32(tc.want.uid), fileInfo.Sys().(*syscall.Stat_t).Uid, tc.name)
			assert.Equal(t, uint32(tc.want.gid), fileInfo.Sys().(*syscall.Stat_t).Gid, tc.name)
		})
	}
}
