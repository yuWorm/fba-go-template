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

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Options struct {
	Bucket         string
	Region         string
	Endpoint       string
	BaseURL        string
	ForcePathStyle bool
}

type S3Config struct {
	ForcePathStyle bool `json:"force_path_style"`
}

type S3 struct {
	bucket    string
	baseURL   string
	client    s3API
	presigner s3PresignAPI
}

type s3API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3PresignAPI interface {
	PresignPutObject(context.Context, *s3.PutObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

func NewS3FromConfig(config BackendConfig) (Backend, error) {
	opts := S3Options{}
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
		var s3Config S3Config
		if err := json.Unmarshal([]byte(*config.Config), &s3Config); err != nil {
			return nil, err
		}
		opts.ForcePathStyle = s3Config.ForcePathStyle
	}
	return NewS3(opts)
}

func NewS3(opts S3Options) (*S3, error) {
	if strings.TrimSpace(opts.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	loadOptions := []func(*awsconfig.LoadOptions) error{}
	if strings.TrimSpace(opts.Region) != "" {
		loadOptions = append(loadOptions, awsconfig.WithRegion(strings.TrimSpace(opts.Region)))
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		if strings.TrimSpace(opts.Endpoint) != "" {
			options.BaseEndpoint = aws.String(strings.TrimSpace(opts.Endpoint))
		}
		options.UsePathStyle = opts.ForcePathStyle
	})
	return NewS3WithClient(opts, client, s3.NewPresignClient(client)), nil
}

func NewS3WithClient(opts S3Options, client s3API, presigner s3PresignAPI) *S3 {
	return &S3{
		bucket:    strings.TrimSpace(opts.Bucket),
		baseURL:   strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		client:    client,
		presigner: presigner,
	}
}

func (b *S3) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error) {
	if b.client == nil {
		return ObjectInfo{}, fmt.Errorf("s3 client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	reader := &countingReader{reader: r}
	output, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(clean),
		Body:        reader,
		ContentType: optionalAWSString(opts.ContentType),
		Metadata:    opts.Metadata,
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: clean, Size: reader.size, ContentType: opts.ContentType, ETag: output.ETag}, nil
}

func (b *S3) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if b.client == nil {
		return nil, ObjectInfo{}, fmt.Errorf("s3 client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	output, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(clean),
	})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return output.Body, ObjectInfo{
		Key:         clean,
		Size:        aws.ToInt64(output.ContentLength),
		ContentType: aws.ToString(output.ContentType),
		ETag:        output.ETag,
	}, nil
}

func (b *S3) Head(ctx context.Context, key string) (ObjectInfo, error) {
	if b.client == nil {
		return ObjectInfo{}, fmt.Errorf("s3 client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	output, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(clean),
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Key:         clean,
		Size:        aws.ToInt64(output.ContentLength),
		ContentType: aws.ToString(output.ContentType),
		ETag:        output.ETag,
	}, nil
}

func (b *S3) Delete(ctx context.Context, key string) error {
	if b.client == nil {
		return fmt.Errorf("s3 client is required")
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return err
	}
	_, err = b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(clean),
	})
	return err
}

func (b *S3) PresignPut(ctx context.Context, key string, ttl time.Duration, opts PutOptions) (PresignedURL, error) {
	if b.presigner == nil {
		return PresignedURL{}, ErrUnsupported
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return PresignedURL{}, err
	}
	request, err := b.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(clean),
		ContentType: optionalAWSString(opts.ContentType),
		Metadata:    opts.Metadata,
	}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return PresignedURL{}, err
	}
	return presignedURLFromS3(request, ttl), nil
}

func (b *S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (PresignedURL, error) {
	if b.presigner == nil {
		return PresignedURL{}, ErrUnsupported
	}
	clean, err := cleanS3Key(key)
	if err != nil {
		return PresignedURL{}, err
	}
	request, err := b.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(clean),
	}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return PresignedURL{}, err
	}
	return presignedURLFromS3(request, ttl), nil
}

func (b *S3) PublicURL(key string) string {
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

type countingReader struct {
	reader io.Reader
	size   int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.size += int64(n)
	return n, err
}

func cleanS3Key(key string) (string, error) {
	clean := strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	if clean == "" || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	clean = path.Clean(clean)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	return clean, nil
}

func optionalAWSString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return aws.String(value)
}

func presignedURLFromS3(request *v4.PresignedHTTPRequest, ttl time.Duration) PresignedURL {
	headers := make(map[string]string, len(request.SignedHeader))
	for key, values := range request.SignedHeader {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return PresignedURL{
		Method:    request.Method,
		URL:       request.URL,
		ExpiresAt: time.Now().Add(ttl),
		Headers:   headers,
	}
}
