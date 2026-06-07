package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
)

func TestOSSBackendPutOpenDeleteAndPublicURL(t *testing.T) {
	client := newFakeOSSClient()
	backend := storage.NewOSSWithClient(storage.OSSOptions{
		Bucket:  "oss-bucket",
		Region:  "cn-hangzhou",
		BaseURL: "https://cdn.example.test/files",
	}, client)

	info, err := backend.Put(context.Background(), "uploads/a file.txt", strings.NewReader("hello"), storage.PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"purpose": "test"},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Key != "uploads/a file.txt" || info.Size != 5 || info.ContentType != "text/plain" || info.ETag == nil {
		t.Fatalf("Put() info = %+v", info)
	}
	if client.lastBucket != "oss-bucket" || client.lastMetadata["purpose"] != "test" {
		t.Fatalf("Put() bucket=%q metadata=%v", client.lastBucket, client.lastMetadata)
	}

	reader, opened, err := backend.Open(context.Background(), "uploads/a file.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(body, []byte("hello")) || opened.Size != 5 || opened.ContentType != "text/plain" {
		t.Fatalf("Open() body=%q info=%+v", body, opened)
	}
	headed, err := backend.Head(context.Background(), "uploads/a file.txt")
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if headed.Size != 5 || headed.ContentType != "text/plain" || headed.ETag == nil {
		t.Fatalf("Head() info = %+v", headed)
	}
	if url := backend.PublicURL("uploads/a file.txt"); url != "https://cdn.example.test/files/uploads/a%20file.txt" {
		t.Fatalf("PublicURL() = %q", url)
	}
	if err := backend.Delete(context.Background(), "uploads/a file.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := backend.Open(context.Background(), "uploads/a file.txt"); err == nil {
		t.Fatal("Open() after Delete succeeded")
	}
}

func TestOSSBackendPresignsPutAndGet(t *testing.T) {
	client := newFakeOSSClient()
	backend := storage.NewOSSWithClient(storage.OSSOptions{
		Bucket: "oss-bucket",
		Region: "cn-hangzhou",
	}, client)

	putURL, err := backend.PresignPut(context.Background(), "uploads/a.txt", 10*time.Minute, storage.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PresignPut() error = %v", err)
	}
	if putURL.Method != http.MethodPut || putURL.URL == "" || putURL.Headers["Content-Type"] != "text/plain" {
		t.Fatalf("PresignPut() = %+v", putURL)
	}
	if client.lastPresignExpires != 10*time.Minute {
		t.Fatalf("PresignPut() ttl = %s", client.lastPresignExpires)
	}

	getURL, err := backend.PresignGet(context.Background(), "uploads/a.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
	if getURL.Method != http.MethodGet || getURL.URL == "" {
		t.Fatalf("PresignGet() = %+v", getURL)
	}
	if client.lastPresignExpires != 15*time.Minute {
		t.Fatalf("PresignGet() ttl = %s", client.lastPresignExpires)
	}
}

func TestOSSFromConfigRequiresBucketAndRegion(t *testing.T) {
	region := "cn-hangzhou"
	bucket := "oss-bucket"
	if _, err := storage.NewOSSFromConfig(storage.BackendConfig{Provider: "oss", Region: &region}); err == nil {
		t.Fatal("NewOSSFromConfig() accepted empty bucket")
	}
	if _, err := storage.NewOSSFromConfig(storage.BackendConfig{Provider: "oss", Bucket: &bucket}); err == nil {
		t.Fatal("NewOSSFromConfig() accepted empty region")
	}
}

func TestRegistryResolvesOSSFactory(t *testing.T) {
	registry := storage.NewRegistry()
	_, ok, err := registry.Resolve(storage.BackendConfig{Provider: "oss"})
	if !ok {
		t.Fatal("Resolve() did not find oss factory")
	}
	if err == nil {
		t.Fatal("Resolve() accepted invalid oss config")
	}
}

type fakeOSSClient struct {
	objects            map[string]fakeOSSObject
	lastBucket         string
	lastMetadata       map[string]string
	lastPresignExpires time.Duration
}

type fakeOSSObject struct {
	body        []byte
	contentType string
	etag        string
	metadata    map[string]string
}

func newFakeOSSClient() *fakeOSSClient {
	return &fakeOSSClient{objects: map[string]fakeOSSObject{}}
}

func (c *fakeOSSClient) PutObject(_ context.Context, input *oss.PutObjectRequest, _ ...func(*oss.Options)) (*oss.PutObjectResult, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	key := ossString(input.Key)
	c.lastBucket = ossString(input.Bucket)
	c.lastMetadata = input.Metadata
	c.objects[key] = fakeOSSObject{
		body:        body,
		contentType: ossString(input.ContentType),
		etag:        `"fake-etag"`,
		metadata:    input.Metadata,
	}
	return &oss.PutObjectResult{ETag: oss.Ptr(`"fake-etag"`)}, nil
}

func (c *fakeOSSClient) GetObject(_ context.Context, input *oss.GetObjectRequest, _ ...func(*oss.Options)) (*oss.GetObjectResult, error) {
	object, ok := c.objects[ossString(input.Key)]
	if !ok {
		return nil, errFakeOSSNotFound
	}
	return &oss.GetObjectResult{
		Body:          io.NopCloser(bytes.NewReader(object.body)),
		ContentLength: int64(len(object.body)),
		ContentType:   oss.Ptr(object.contentType),
		ETag:          oss.Ptr(object.etag),
		Metadata:      object.metadata,
	}, nil
}

func (c *fakeOSSClient) HeadObject(_ context.Context, input *oss.HeadObjectRequest, _ ...func(*oss.Options)) (*oss.HeadObjectResult, error) {
	object, ok := c.objects[ossString(input.Key)]
	if !ok {
		return nil, errFakeOSSNotFound
	}
	return &oss.HeadObjectResult{
		ContentLength: int64(len(object.body)),
		ContentType:   oss.Ptr(object.contentType),
		ETag:          oss.Ptr(object.etag),
		Metadata:      object.metadata,
	}, nil
}

func (c *fakeOSSClient) DeleteObject(_ context.Context, input *oss.DeleteObjectRequest, _ ...func(*oss.Options)) (*oss.DeleteObjectResult, error) {
	delete(c.objects, ossString(input.Key))
	return &oss.DeleteObjectResult{}, nil
}

func (c *fakeOSSClient) Presign(_ context.Context, request any, optFns ...func(*oss.PresignOptions)) (*oss.PresignResult, error) {
	options := oss.PresignOptions{}
	for _, fn := range optFns {
		fn(&options)
	}
	c.lastPresignExpires = options.Expires
	expiration := time.Now().Add(options.Expires)
	switch input := request.(type) {
	case *oss.PutObjectRequest:
		return &oss.PresignResult{
			Method:     http.MethodPut,
			URL:        "https://signed.example.test/" + ossString(input.Key),
			Expiration: expiration,
			SignedHeaders: map[string]string{
				"Content-Type": ossString(input.ContentType),
			},
		}, nil
	case *oss.GetObjectRequest:
		return &oss.PresignResult{
			Method:     http.MethodGet,
			URL:        "https://signed.example.test/" + ossString(input.Key),
			Expiration: expiration,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported presign request %T", request)
	}
}

func ossString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var errFakeOSSNotFound = &fakeOSSError{}

type fakeOSSError struct{}

func (*fakeOSSError) Error() string {
	return "fake oss object not found"
}
