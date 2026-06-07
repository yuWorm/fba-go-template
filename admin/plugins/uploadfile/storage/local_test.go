package storage_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
)

func TestLocalBackendWritesReadsDeletesAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	backend := storage.NewLocal(storage.LocalOptions{Root: root, BaseURL: "https://cdn.example.test/assets"})
	ctx := context.Background()

	info, err := backend.Put(ctx, "uploads/default/file.txt", strings.NewReader("hello"), storage.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Key != "uploads/default/file.txt" || info.Size != 5 || info.ContentType != "text/plain" {
		t.Fatalf("Put() info = %+v", info)
	}
	if backend.PublicURL(info.Key) != "https://cdn.example.test/assets/uploads/default/file.txt" {
		t.Fatalf("PublicURL() = %q", backend.PublicURL(info.Key))
	}

	reader, opened, err := backend.Open(ctx, info.Key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(body, []byte("hello")) || opened.Size != 5 {
		t.Fatalf("opened body=%q info=%+v", body, opened)
	}

	if err := backend.Delete(ctx, info.Key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := backend.Open(ctx, info.Key); err == nil {
		t.Fatal("Open() after Delete succeeded, want error")
	}

	if _, err := backend.Put(ctx, "../escape.txt", strings.NewReader("bad"), storage.PutOptions{}); err == nil {
		t.Fatal("Put() traversal key succeeded, want error")
	}
	if _, _, err := backend.Open(ctx, "/absolute.txt"); err == nil {
		t.Fatal("Open() absolute key succeeded, want error")
	}
}
