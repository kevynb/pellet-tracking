package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"pellets-tracker/internal/core"
)

const (
	backupSuffix   = ".bak"
	maxBackupFiles = 3
	filePerms      = 0o600
	dirPerms       = 0o755
)

// JSONStore manages concurrent access to a JSON-backed datastore.
type JSONStore struct {
	path      string
	backupDir string

	mu   sync.RWMutex
	data *core.DataStore
}

// NewJSONStore loads the datastore from disk or initializes a new one when the
// file does not exist.
func NewJSONStore(path, backupDir string) (*JSONStore, error) {
	data, err := Load(path)
	if err != nil {
		return nil, err
	}

	if backupDir == "" {
		backupDir = filepath.Dir(path)
	}

	if err := os.MkdirAll(backupDir, dirPerms); err != nil {
		return nil, fmt.Errorf("ensure backup dir: %w", err)
	}

	return &JSONStore{path: path, backupDir: backupDir, data: data}, nil
}

// Data returns a deep copy of the current datastore snapshot.
func (s *JSONStore) Data() core.DataStore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneDataStore(s.data)
}

// Replace swaps the in-memory datastore with the provided snapshot and persists it.
func (s *JSONStore) Replace(data core.DataStore) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := cloneDataStore(&data)
	s.data = &cloned
	return Save(s.path, s.backupDir, s.data)
}

// Load reads a datastore from disk. When the file does not exist a new
// datastore is returned with initialized metadata.
func Load(path string) (*core.DataStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), dirPerms); err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			now := time.Now().UTC()
			return &core.DataStore{Meta: core.Meta{ID: core.NewID(), CreatedAt: now, UpdatedAt: now}}, nil
		}
		return nil, fmt.Errorf("open datastore: %w", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	var ds core.DataStore
	if err := decoder.Decode(&ds); err != nil {
		return nil, fmt.Errorf("decode datastore: %w", err)
	}

	if ds.Meta.ID == "" {
		ds.Meta.ID = core.NewID()
	}
	if ds.Meta.CreatedAt.IsZero() {
		ds.Meta.CreatedAt = time.Now().UTC()
	}
	ds.Meta.UpdatedAt = time.Now().UTC()

	return &ds, nil
}

// Save persists the datastore to disk with pretty-printed JSON, creating a
// rotated backup beforehand.
func Save(path, backupDir string, data *core.DataStore) error {
	if data == nil {
		return fmt.Errorf("nil datastore")
	}

	data.Meta.UpdatedAt = time.Now().UTC()

	if err := Backup(path, backupDir); err != nil {
		return fmt.Errorf("backup datastore: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), "datastore-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode datastore: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, filePerms); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Backup creates a backup of the datastore file before writing a new version,
// keeping only the latest maxBackupFiles copies.
func Backup(path, backupDir string) error {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat datastore: %w", err)
	}

	if backupDir == "" {
		backupDir = filepath.Dir(path)
	}

	if err := os.MkdirAll(backupDir, dirPerms); err != nil {
		return fmt.Errorf("ensure backup dir: %w", err)
	}

	base := filepath.Base(path)
	name := fmt.Sprintf("%s-%s%s", base, time.Now().UTC().Format("20060102T150405Z"), backupSuffix)
	backupPath := filepath.Join(backupDir, name)

	if err := copyFile(path, backupPath); err != nil {
		return fmt.Errorf("copy backup: %w", err)
	}

	if err := os.Chmod(backupPath, filePerms); err != nil {
		return fmt.Errorf("chmod backup: %w", err)
	}

	pattern := fmt.Sprintf("%s-%s%s", base, "*", backupSuffix)
	matches, err := filepath.Glob(filepath.Join(backupDir, pattern))
	if err != nil {
		return fmt.Errorf("glob backups: %w", err)
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i] > matches[j] })

	for idx, file := range matches {
		if idx < maxBackupFiles {
			continue
		}
		_ = os.Remove(file)
	}

	return nil
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, filePerms)
}

func cloneDataStore(ds *core.DataStore) core.DataStore {
	if ds == nil {
		return core.DataStore{}
	}
	clone := *ds
	clone.Brands = append([]core.Brand(nil), ds.Brands...)
	clone.Purchases = append([]core.Purchase(nil), ds.Purchases...)
	clone.Consumptions = append([]core.Consumption(nil), ds.Consumptions...)
	return clone
}
