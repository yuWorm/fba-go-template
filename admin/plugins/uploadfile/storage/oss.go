package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type OSSOptions struct {
	Bucket       string
	Region       string
	Endpoint     string
	BaseURL      string
	UsePathStyle bool
	UseCName     bool
}

type OSSConfig struct {
	UsePathStyle bool `json:"use_path_style"`
	UseCName     bool `json:"use_cname"`
}

type OSS struct {
	bucket  string
	baseURL string
	client  ossAPI
}

type ossAPI interface {
	PutObject(context.Context, *alioss.PutObjectRequest, ...func(*alioss.Options)) (*alioss.PutObjectResult, error)
	GetObject(context.Context, *alioss.GetObjectRequest, ...func(*alioss.Options)) (*alioss.GetObjectResult, error)
	DeleteObject(context.Context, *alioss.DeleteObjectRequest, ...func(*alioss.Options)) (*alioss.DeleteObjectResult, error)
	Presign(context.Context, any, ...func(*alioss.PresignOptions)) (*alioss.PresignResult, error)
}

func NewOSSFromConfig(config BackendConfig) (Backend, error) {
	opts := OSSOptions{}
	if config.Bucket != nil {
		opts.Bucket = strings.TrimSpace(*config.Bucket)
	}
	if config.Region != nil {
		opts.Region = strings.TrimSpace(*config.Region)
	}
	if config.Endpoint != nil {
		opts.Endpoint = strings.TrimSpace(*config.Endpoint)
	}
	if config.BaseURL != nil {
		opts.BaseURL = strings.TrimSpace(*config.BaseURL)
	}
	if config.Config != nil && strings.TrimSpace(*config.Config) != "" {
		var ossConfig OSSConfig
		if err := json.Unmarshal([]byte(*config.Config), &ossConfig); err != nil {
			return nil, err
		}
		opts.UsePathStyle = ossConfig.UsePathStyle
		opts.UseCName = ossConfig.UseCName
	}
	return NewOSS(opts)
}

func NewOSS(opts OSSOptions) (*OSS, error) {
	opts.Bucket = strings.TrimSpace(opts.Bucket)
	opts.Region = strings.TrimSpace(opts.Region)
	opts.Endpoint = strings.TrimSpace(opts.Endpoint)
	opts.BaseURL = strings.TrimSpace(opts.BaseURL)
	if opts.Bucket == "" {
		return nil, fmt.Errorf("oss bucket is required")
	}
	if opts.Region == "" {
		return nil, fmt.Errorf("oss region is required")
	}
	cfg := alioss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(opts.Region)
	if opts.Endpoint != "" {
		cfg = cfg.WithEndpoint(opts.Endpoint)
	}
	if opts.UsePathStyle {
		cfg = cfg.WithUsePathStyle(true)
	}
	if opts.UseCName {
		cfg = cfg.WithUseCName(true)
	}
	return NewOSSWithClient(opts, alioss.NewClient(cfg)), nil
}

func NewOSSWithClient(opts OSSOptions, client ossAPI) *OSS {
	return &OSS{
		bucket:  strings.TrimSpace(opts.Bucket),
		baseURL: strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		client:  client,
	}
}

func (b *OSS) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error) {
	if b.client == nil {
		return ObjectInfo{}, fmt.Errorf("oss client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	reader := &countingReader{reader: r}
	output, err := b.client.PutObject(ctx, &alioss.PutObjectRequest{
		Bucket:      alioss.Ptr(b.bucket),
		Key:         alioss.Ptr(clean),
		Body:        reader,
		ContentType: optionalOSSString(opts.ContentType),
		Metadata:    opts.Metadata,
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: clean, Size: reader.size, ContentType: opts.ContentType, ETag: output.ETag}, nil
}

func (b *OSS) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if b.client == nil {
		return nil, ObjectInfo{}, fmt.Errorf("oss client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	output, err := b.client.GetObject(ctx, &alioss.GetObjectRequest{
		Bucket: alioss.Ptr(b.bucket),
		Key:    alioss.Ptr(clean),
	})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return output.Body, ObjectInfo{
		Key:         clean,
		Size:        output.ContentLength,
		ContentType: ossString(output.ContentType),
		ETag:        output.ETag,
	}, nil
}

func (b *OSS) Delete(ctx context.Context, key string) error {
	if b.client == nil {
		return fmt.Errorf("oss client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return err
	}
	_, err = b.client.DeleteObject(ctx, &alioss.DeleteObjectRequest{
		Bucket: alioss.Ptr(b.bucket),
		Key:    alioss.Ptr(clean),
	})
	return err
}

func (b *OSS) PresignPut(ctx context.Context, key string, ttl time.Duration, opts PutOptions) (PresignedURL, error) {
	if b.client == nil {
		return PresignedURL{}, ErrUnsupported
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return PresignedURL{}, err
	}
	request, err := b.client.Presign(ctx, &alioss.PutObjectRequest{
		Bucket:      alioss.Ptr(b.bucket),
		Key:         alioss.Ptr(clean),
		ContentType: optionalOSSString(opts.ContentType),
		Metadata:    opts.Metadata,
	}, alioss.PresignExpires(ttl))
	if err != nil {
		return PresignedURL{}, err
	}
	return presignedURLFromOSS(request, ttl), nil
}

func (b *OSS) PresignGet(ctx context.Context, key string, ttl time.Duration) (PresignedURL, error) {
	if b.client == nil {
		return PresignedURL{}, ErrUnsupported
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return PresignedURL{}, err
	}
	request, err := b.client.Presign(ctx, &alioss.GetObjectRequest{
		Bucket: alioss.Ptr(b.bucket),
		Key:    alioss.Ptr(clean),
	}, alioss.PresignExpires(ttl))
	if err != nil {
		return PresignedURL{}, err
	}
	return presignedURLFromOSS(request, ttl), nil
}

func (b *OSS) PublicURL(key string) string {
	if b.baseURL == "" {
		return ""
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return ""
	}
	encoded := strings.TrimLeft(path.Clean("/"+clean), "/")
	return b.baseURL + "/" + (&url.URL{Path: encoded}).EscapedPath()
}

func optionalOSSString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return alioss.Ptr(value)
}

func ossString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func presignedURLFromOSS(request *alioss.PresignResult, ttl time.Duration) PresignedURL {
	headers := make(map[string]string, len(request.SignedHeaders))
	for key, value := range request.SignedHeaders {
		headers[key] = value
	}
	expiresAt := request.Expiration
	if expiresAt.IsZero() && ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	return PresignedURL{
		Method:    request.Method,
		URL:       request.URL,
		ExpiresAt: expiresAt,
		Headers:   headers,
	}
}
