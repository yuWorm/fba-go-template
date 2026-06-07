package storage_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
)

func TestS3BackendPutOpenDeleteAndPublicURL(t *testing.T) {
	client := newFakeS3Client()
	backend := storage.NewS3WithClient(storage.S3Options{
		Bucket:  "bucket-a",
		BaseURL: "https://cdn.example.test/files",
	}, client, fakeS3Presigner{})

	info, err := backend.Put(context.Background(), "uploads/a file.txt", strings.NewReader("hello"), storage.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Key != "uploads/a file.txt" || info.Size != 5 || info.ContentType != "text/plain" || info.ETag == nil {
		t.Fatalf("Put() info = %+v", info)
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

func TestS3BackendPresignsPutAndGet(t *testing.T) {
	backend := storage.NewS3WithClient(storage.S3Options{
		Bucket: "bucket-a",
	}, newFakeS3Client(), fakeS3Presigner{})

	putURL, err := backend.PresignPut(context.Background(), "uploads/a.txt", 10*time.Minute, storage.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PresignPut() error = %v", err)
	}
	if putURL.Method != http.MethodPut || putURL.URL == "" || putURL.Headers["Content-Type"] != "text/plain" {
		t.Fatalf("PresignPut() = %+v", putURL)
	}
	getURL, err := backend.PresignGet(context.Background(), "uploads/a.txt", 10*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
	if getURL.Method != http.MethodGet || getURL.URL == "" {
		t.Fatalf("PresignGet() = %+v", getURL)
	}
}

func TestS3FromConfigRequiresBucket(t *testing.T) {
	if _, err := storage.NewS3FromConfig(storage.BackendConfig{Provider: "s3"}); err == nil {
		t.Fatal("NewS3FromConfig() accepted empty bucket")
	}
}

type fakeS3Client struct {
	objects map[string]fakeS3Object
}

type fakeS3Object struct {
	body        []byte
	contentType string
	etag        string
}

func newFakeS3Client() *fakeS3Client {
	return &fakeS3Client{objects: map[string]fakeS3Object{}}
}

func (c *fakeS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	c.objects[aws.ToString(input.Key)] = fakeS3Object{
		body:        body,
		contentType: aws.ToString(input.ContentType),
		etag:        `"fake-etag"`,
	}
	return &s3.PutObjectOutput{ETag: aws.String(`"fake-etag"`)}, nil
}

func (c *fakeS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	object, ok := c.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, errFakeS3NotFound
	}
	size := int64(len(object.body))
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(object.body)),
		ContentLength: &size,
		ContentType:   aws.String(object.contentType),
		ETag:          aws.String(object.etag),
	}, nil
}

func (c *fakeS3Client) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	object, ok := c.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, errFakeS3NotFound
	}
	size := int64(len(object.body))
	return &s3.HeadObjectOutput{
		ContentLength: &size,
		ContentType:   aws.String(object.contentType),
		ETag:          aws.String(object.etag),
	}, nil
}

func (c *fakeS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(c.objects, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}

type fakeS3Presigner struct{}

func (fakeS3Presigner) PresignPutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{
		Method:       http.MethodPut,
		URL:          "https://signed.example.test/" + aws.ToString(input.Key),
		SignedHeader: http.Header{"Content-Type": []string{aws.ToString(input.ContentType)}},
	}, nil
}

func (fakeS3Presigner) PresignGetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{
		Method: http.MethodGet,
		URL:    "https://signed.example.test/" + aws.ToString(input.Key),
	}, nil
}

var errFakeS3NotFound = &fakeS3Error{}

type fakeS3Error struct{}

func (*fakeS3Error) Error() string {
	return "fake s3 object not found"
}
