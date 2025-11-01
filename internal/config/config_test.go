package config

import (
	"os"
	"path/filepath"
	"sync"
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
