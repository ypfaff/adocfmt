package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// download returns the file at url, which must match integrity, a hash as
// package-lock.json writes it ("sha512-" and the digest in base64). It keeps a
// verified copy in cacheDir, so a machine fetches each pinned file only once.
func download(cacheDir, url, integrity string) ([]byte, error) {
	want, err := sha512Digest(integrity)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	cached := filepath.Join(cacheDir, hex.EncodeToString(want), path.Base(url))
	if data, err := os.ReadFile(cached); err == nil && verify(data, want) == nil {
		return data, nil
	}

	log.Printf("downloading %s", url)
	// A stalled connection would otherwise hang the build until CI kills it.
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	if err := verify(data, want); err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}

	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cached, data, 0o644); err != nil {
		return nil, err
	}
	return data, nil
}

func sha512Digest(integrity string) ([]byte, error) {
	encoded, ok := strings.CutPrefix(integrity, "sha512-")
	if !ok {
		return nil, fmt.Errorf("integrity %q is not a SHA-512 hash", integrity)
	}
	digest, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(digest) != sha512.Size {
		return nil, fmt.Errorf("integrity %q is not a SHA-512 hash", integrity)
	}
	return digest, nil
}

func verify(data, want []byte) error {
	if got := sha512.Sum512(data); !bytes.Equal(got[:], want) {
		return fmt.Errorf("SHA-512 %x, want %x", got, want)
	}
	return nil
}
