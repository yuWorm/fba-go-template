package storage

import (
	"context"
	"errors"
	"io"
	"strings"
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

type BackendConfig struct {
	Code     string
	Provider string
	Bucket   *string
	Region   *string
	Endpoint *string
	BaseURL  *string
	Prefix   string
	Config   *string
}

type Factory func(config BackendConfig) (Backend, error)

type Registry struct {
	backends  map[string]Backend
	factories map[string]Factory
}

func NewRegistry() *Registry {
	registry := &Registry{backends: map[string]Backend{}, factories: map[string]Factory{}}
	registry.AddFactory("local", NewLocalFromConfig)
	registry.AddFactory("s3", NewS3FromConfig)
	registry.AddFactory("oss", NewOSSFromConfig)
	return registry
}

func (r *Registry) Add(code string, backend Backend) {
	if r.backends == nil {
		r.backends = map[string]Backend{}
	}
	r.backends[code] = backend
}

func (r *Registry) AddFactory(provider string, factory Factory) {
	if r.factories == nil {
		r.factories = map[string]Factory{}
	}
	provider = strings.TrimSpace(provider)
	if provider == "" || factory == nil {
		return
	}
	r.factories[provider] = factory
}

func (r *Registry) Get(code string) (Backend, bool) {
	if r == nil {
		return nil, false
	}
	backend, ok := r.backends[code]
	return backend, ok
}

func (r *Registry) Resolve(config BackendConfig) (Backend, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	code := strings.TrimSpace(config.Code)
	if code != "" {
		if backend, ok := r.Get(code); ok {
			return backend, true, nil
		}
	}
	factory, ok := r.factories[strings.TrimSpace(config.Provider)]
	if !ok {
		return nil, false, nil
	}
	backend, err := factory(config)
	if err != nil {
		return nil, true, err
	}
	return backend, true, nil
}
