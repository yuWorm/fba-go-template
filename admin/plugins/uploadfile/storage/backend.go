package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrUnsupported = errors.New("unsupported storage operation")

type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ETag        *string
}

type PresignedURL struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	ExpiresAt time.Time         `json:"expires_at"`
	Headers   map[string]string `json:"headers"`
}

type Backend interface {
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	PresignPut(ctx context.Context, key string, ttl time.Duration, opts PutOptions) (PresignedURL, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (PresignedURL, error)
	PublicURL(key string) string
}

type Registry struct {
	backends map[string]Backend
}

func NewRegistry() *Registry {
	return &Registry{backends: map[string]Backend{}}
}

func (r *Registry) Add(code string, backend Backend) {
	if r.backends == nil {
		r.backends = map[string]Backend{}
	}
	r.backends[code] = backend
}

func (r *Registry) Get(code string) (Backend, bool) {
	if r == nil {
		return nil, false
	}
	backend, ok := r.backends[code]
	return backend, ok
}
