package credstore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrNotFound = errors.New("not found")

const (
	KeyDeviceToken = "device-token"
	KeyHMAC        = "source-hmac-key"
	KeyDeviceID    = "device-id"
	KeyLogin       = "user-login"
)

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

type FileStore struct {
	Dir string
}

func DirStore(dir string) *FileStore {
	return &FileStore{Dir: dir}
}

func (s *FileStore) path(key string) string {
	return filepath.Join(s.Dir, strings.ReplaceAll(key, "/", "_"))
}

func (s *FileStore) Get(key string) (string, error) {
	raw, err := os.ReadFile(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	v := strings.TrimSpace(string(raw))
	if v == "" {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *FileStore) Set(key, value string) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path(key), []byte(value+"\n"), 0o600)
}

func (s *FileStore) Delete(key string) error {
	err := os.Remove(s.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func Open(configDir string) Store {
	file := DirStore(filepath.Join(configDir, "hosted"))
	if osb := osBackend(); osb != nil {
		return &chain{os: osb, file: file}
	}
	return file
}

func IsUnix() bool { return runtime.GOOS != "windows" }
