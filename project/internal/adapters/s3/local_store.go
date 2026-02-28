package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type LocalStore struct {
	RootDir      string
	DownloadBase string
}

func NewLocalStore(rootDir, downloadBase string) *LocalStore {
	return &LocalStore{RootDir: rootDir, DownloadBase: downloadBase}
}

func (s *LocalStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	path := filepath.Join(s.RootDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.RootDir, filepath.FromSlash(key)))
}

func (s *LocalStore) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if s.DownloadBase == "" {
		return "", fmt.Errorf("download base URL is not configured")
	}
	return fmt.Sprintf("%s?key=%s", s.DownloadBase, key), nil
}
