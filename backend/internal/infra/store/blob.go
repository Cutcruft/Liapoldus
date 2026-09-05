package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// DiskBlobStore persists raw asset bytes under assetDir/<site-id>/<asset-id>.
// Writes are atomic (temp file + rename) and isolated per site.
type DiskBlobStore struct {
	baseDir string
}

func NewDiskBlobStore(baseDir string) (*DiskBlobStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create asset dir %q: %w", baseDir, err)
	}
	return &DiskBlobStore{baseDir: baseDir}, nil
}

func (b *DiskBlobStore) blobPath(siteID, assetID string) (string, error) {
	if siteID == "" || assetID == "" || filepath.Base(siteID) != siteID || filepath.Base(assetID) != assetID {
		return "", fmt.Errorf("%w: invalid asset id for disk path", domain.ErrInvalidRequest)
	}
	return filepath.Join(b.baseDir, siteID, assetID), nil
}

func (b *DiskBlobStore) Save(_ context.Context, siteID, assetID string, data io.Reader) (int64, string, error) {
	path, err := b.blobPath(siteID, assetID)
	if err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, "", fmt.Errorf("create asset site dir: %w", err)
	}
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return 0, "", fmt.Errorf("create asset temp file: %w", err)
	}
	hash := sha1.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), data)
	if copyErr == nil {
		copyErr = file.Sync()
	}
	if closeErr := file.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.Remove(tmp)
		return 0, "", fmt.Errorf("write asset data: %w", copyErr)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return 0, "", fmt.Errorf("finalize asset: %w", err)
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func (b *DiskBlobStore) Open(_ context.Context, siteID, assetID string) (io.ReadCloser, error) {
	path, err := b.blobPath(siteID, assetID)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("open asset: %w", err)
	}
	return file, nil
}

func (b *DiskBlobStore) Delete(_ context.Context, siteID, assetID string) error {
	path, err := b.blobPath(siteID, assetID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete asset: %w", err)
	}
	return nil
}
