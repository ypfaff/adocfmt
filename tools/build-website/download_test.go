package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownload(t *testing.T) {
	content := []byte("pagefind")
	sum := sha512.Sum512(content)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write(content)
	}))
	defer server.Close()
	cacheDir := t.TempDir()

	for range 2 {
		got, err := download(cacheDir, server.URL+"/file", integrity)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("got %q, want %q", got, content)
		}
	}
	if requests != 1 {
		t.Errorf("got %d requests, want 1: the second download comes from the cache", requests)
	}

	wrong := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, sha512.Size))
	if _, err := download(t.TempDir(), server.URL+"/file", wrong); err == nil {
		t.Error("a file that does not match its integrity was accepted")
	}
	if _, err := download(t.TempDir(), server.URL+"/file", "sha1-AAAA"); err == nil {
		t.Error("an integrity that is not SHA-512 was accepted")
	}
}

func TestReadLockfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "package-lock.json")
	lock := `{"packages": {
		"": {"name": "website"},
		"node_modules/@pagefind/linux-x64": {"version": "1.5.2", "resolved": "https://example.com/linux-x64.tgz", "integrity": "sha512-x"}
	}}`
	if err := os.WriteFile(path, []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}

	packages, err := readLockfile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := npmPackage{Version: "1.5.2", Resolved: "https://example.com/linux-x64.tgz", Integrity: "sha512-x"}
	if len(packages) != 1 || packages["@pagefind/linux-x64"] != want {
		t.Errorf("got %v, want only @pagefind/linux-x64: %v", packages, want)
	}
}

func TestExtract(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"package/README.md": "readme", "package/bin/pagefind": "binary"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extract(archive.Bytes(), "package/bin/pagefind")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary" {
		t.Errorf("got %q, want %q", got, "binary")
	}
	if _, err := extract(archive.Bytes(), "package/bin/pagefind.exe"); err == nil {
		t.Error("got no error for a file the archive does not hold")
	}
}
