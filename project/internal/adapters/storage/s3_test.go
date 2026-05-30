package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"diplom.com/m/internal/ports"
)

func TestS3StoreSavePutsObject(t *testing.T) {
	var gotPath, gotAuth, gotHash, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		gotHash = r.Header.Get("X-Amz-Content-Sha256")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store, err := NewS3Store(srv.URL, "docsapp", "us-east-1", "access", "secret")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Client = srv.Client()

	n, err := store.Save(context.Background(), "plan_item_1/doc.pdf", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes saved, got %d", n)
	}
	if gotPath != "/docsapp/plan_item_1/doc.pdf" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if gotBody != "hello" {
		t.Fatalf("unexpected body: %q", gotBody)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=access/") {
		t.Fatalf("missing AWS4 authorization header: %q", gotAuth)
	}
	if gotHash == "" {
		t.Fatalf("missing payload hash")
	}
}

func TestS3StoreSaveReturnsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no bucket", http.StatusNotFound)
	}))
	defer srv.Close()

	store, err := NewS3Store(srv.URL, "docsapp", "us-east-1", "access", "secret")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Client = srv.Client()

	if _, err := store.Save(context.Background(), "doc.pdf", strings.NewReader("hello")); err == nil {
		t.Fatalf("expected save error")
	}
}

func TestS3StoreStatHeadsObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("expected HEAD, got %s", r.Method)
		}
		if r.URL.EscapedPath() != "/docsapp/doc.pdf" {
			t.Fatalf("unexpected path: %q", r.URL.EscapedPath())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=access/") {
			t.Fatalf("missing AWS4 authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Length", "123")
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("ETag", `"etag"`)
		w.Header().Set("Last-Modified", "Sat, 30 May 2026 13:04:11 GMT")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store, err := NewS3Store(srv.URL, "docsapp", "us-east-1", "access", "secret")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Client = srv.Client()

	status, err := store.Stat(context.Background(), "doc.pdf")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !status.Exists || status.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.ContentLength == nil || *status.ContentLength != 123 {
		t.Fatalf("unexpected content length: %v", status.ContentLength)
	}
	if status.ContentType != "application/pdf" || status.ETag != `"etag"` || status.LastModified == nil {
		t.Fatalf("unexpected headers: %+v", status)
	}
}

func TestS3StoreStatMissingObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store, err := NewS3Store(srv.URL, "docsapp", "us-east-1", "access", "secret")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Client = srv.Client()

	status, err := store.Stat(context.Background(), "missing.pdf")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if status.Exists || status.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected missing status: %+v", status)
	}
}

func TestS3StoreOpenGetsObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.EscapedPath() != "/docsapp/doc.txt" {
			t.Fatalf("unexpected path: %q", r.URL.EscapedPath())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=access/") {
			t.Fatalf("missing AWS4 authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("ETag", `"etag"`)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	store, err := NewS3Store(srv.URL, "docsapp", "us-east-1", "access", "secret")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Client = srv.Client()

	obj, err := store.Open(context.Background(), "doc.txt")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer obj.Body.Close()
	body, err := io.ReadAll(obj.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if !obj.Status.Exists || obj.Status.ContentType != "text/plain" {
		t.Fatalf("unexpected object status: %+v", obj.Status)
	}
}

func TestS3StoreOpenMissingObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store, err := NewS3Store(srv.URL, "docsapp", "us-east-1", "access", "secret")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Client = srv.Client()

	if _, err := store.Open(context.Background(), "missing.txt"); err != ports.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
