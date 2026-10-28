package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
)

// DiskCache implements port.DiskCache for persistent disk caching.
type DiskCache struct {
	baseDir     string
	catalogDir  string
	archiveDir  string
	defaultTTL  time.Duration
	mu          sync.RWMutex
}

var _ port.DiskCache = (*DiskCache)(nil)

// NewDiskCache creates a disk cache under baseDir.
// If baseDir is empty, it uses ~/.cache/isa-afp-mcp.
func NewDiskCache(baseDir string, defaultTTL time.Duration) (*DiskCache, error) {
	if baseDir == "" {
		cacheHome := os.Getenv("XDG_CACHE_HOME")
		if cacheHome == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("resolve user home dir: %w", err)
			}
			cacheHome = filepath.Join(home, ".cache")
		}
		baseDir = filepath.Join(cacheHome, "isa-afp-mcp")
	}

	catalogDir := filepath.Join(baseDir, "catalog")
	archiveDir := filepath.Join(baseDir, "archives")

	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		return nil, fmt.Errorf("create catalog dir: %w", err)
	}
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, fmt.Errorf("create archives dir: %w", err)
	}

	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}

	return &DiskCache{
		baseDir:    baseDir,
		catalogDir: catalogDir,
		archiveDir: archiveDir,
		defaultTTL: defaultTTL,
	}, nil
}

func (d *DiskCache) keyPath(key string) string {
	// Sanitize key name or hash if it contains path characters
	hash := sha256.Sum256([]byte(key))
	safeName := hex.EncodeToString(hash[:8]) + "_" + filepath.Base(key)
	return filepath.Join(d.catalogDir, safeName)
}

// Get reads cached bytes if exists and not expired.
func (d *DiskCache) Get(key string) ([]byte, bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p := d.keyPath(key)
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if time.Since(info.ModTime()) > d.defaultTTL {
		return nil, false, nil // Expired
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Set writes data to disk atomically.
func (d *DiskCache) Set(key string, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	p := d.keyPath(key)
	tmpPath := fmt.Sprintf("%s.tmp.%d", p, time.Now().UnixNano())

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp cache file: %w", err)
	}

	if err := os.Rename(tmpPath, p); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic rename cache file: %w", err)
	}
	return nil
}

// Has checks if a key exists and is fresh.
func (d *DiskCache) Has(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p := d.keyPath(key)
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) <= d.defaultTTL
}

// GetArchivePath returns the local path for an entry's cached .tar.gz archive.
func (d *DiskCache) GetArchivePath(entryShortname string) string {
	return filepath.Join(d.archiveDir, fmt.Sprintf("afp-%s.tar.gz", entryShortname))
}

// SaveArchive streams the package archive to disk atomically.
func (d *DiskCache) SaveArchive(entryShortname string, r io.Reader) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	targetPath := d.GetArchivePath(entryShortname)
	tmpPath := fmt.Sprintf("%s.tmp.%d", targetPath, time.Now().UnixNano())

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("create archive tmp file: %w", err)
	}

	_, err = io.Copy(f, r)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write archive data: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close archive file: %w", closeErr)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("atomic rename archive: %w", err)
	}

	return targetPath, nil
}

// OpenArchive opens a previously cached package archive.
func (d *DiskCache) OpenArchive(entryShortname string) (io.ReadCloser, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	targetPath := d.GetArchivePath(entryShortname)
	f, err := os.Open(targetPath)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Clear removes all cached items.
func (d *DiskCache) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_ = os.RemoveAll(d.catalogDir)
	_ = os.RemoveAll(d.archiveDir)
	_ = os.MkdirAll(d.catalogDir, 0755)
	_ = os.MkdirAll(d.archiveDir, 0755)
	return nil
}
