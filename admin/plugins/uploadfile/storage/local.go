package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type LocalOptions struct {
	Root    string
	BaseURL string
}

type LocalConfig struct {
	Root    string `json:"root"`
	BaseURL string `json:"base_url"`
}

type Local struct {
	root    string
	baseURL string
}

func NewLocal(opts LocalOptions) *Local {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = ".cache/uploadfile"
	}
	return &Local{root: root, baseURL: strings.TrimRight(opts.BaseURL, "/")}
}

func NewLocalFromConfig(config BackendConfig) (Backend, error) {
	var localConfig LocalConfig
	if config.Config != nil && strings.TrimSpace(*config.Config) != "" {
		if err := json.Unmarshal([]byte(*config.Config), &localConfig); err != nil {
			return nil, err
		}
	}
	baseURL := localConfig.BaseURL
	if config.BaseURL != nil && strings.TrimSpace(*config.BaseURL) != "" {
		baseURL = strings.TrimSpace(*config.BaseURL)
	}
	return NewLocal(LocalOptions{Root: localConfig.Root, BaseURL: baseURL}), nil
}

func (b *Local) Put(_ context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error) {
	target, clean, err := b.resolve(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return ObjectInfo{}, err
	}
	file, err := os.Create(target)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer file.Close()
	size, err := io.Copy(file, r)
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: clean, Size: size, ContentType: opts.ContentType}, nil
}

func (b *Local) Open(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	target, clean, err := b.resolve(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, ObjectInfo{Key: clean, Size: stat.Size()}, nil
}

func (b *Local) Head(_ context.Context, key string) (ObjectInfo, error) {
	target, clean, err := b.resolve(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	stat, err := os.Stat(target)
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: clean, Size: stat.Size()}, nil
}

func (b *Local) Delete(_ context.Context, key string) error {
	target, _, err := b.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (b *Local) PresignPut(context.Context, string, time.Duration, PutOptions) (PresignedURL, error) {
	return PresignedURL{}, ErrUnsupported
}

func (b *Local) PresignGet(context.Context, string, time.Duration) (PresignedURL, error) {
	return PresignedURL{}, ErrUnsupported
}

func (b *Local) PublicURL(key string) string {
	_, clean, err := b.resolve(key)
	if err != nil {
		return ""
	}
	if b.baseURL == "" {
		return "/" + clean
	}
	encoded := strings.TrimLeft(path.Clean("/"+clean), "/")
	return b.baseURL + "/" + (&url.URL{Path: encoded}).EscapedPath()
}

func (b *Local) resolve(key string) (string, string, error) {
	clean := strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	if clean == "" || strings.HasPrefix(clean, "/") {
		return "", "", fmt.Errorf("invalid object key %q", key)
	}
	clean = path.Clean(clean)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", "", fmt.Errorf("invalid object key %q", key)
	}
	root, err := filepath.Abs(b.root)
	if err != nil {
		return "", "", err
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("invalid object key %q", key)
	}
	return target, clean, nil
}
