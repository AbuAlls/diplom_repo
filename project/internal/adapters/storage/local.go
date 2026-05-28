package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"diplom.com/m/internal/ports"
)

// LocalStore persists uploaded files on the local filesystem rooted at RootDir.
type LocalStore struct {
	RootDir string
}

func NewLocalStore(rootDir string) *LocalStore { return &LocalStore{RootDir: rootDir} }

func (s *LocalStore) Save(_ context.Context, key string, r io.Reader) (int64, error) {
	path := filepath.Join(s.RootDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

var _ ports.FileStore = (*LocalStore)(nil)
