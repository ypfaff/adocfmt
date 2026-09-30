package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// lockfile is website/package-lock.json. The build takes its tools from the
// packages it pins, so Dependabot updates them and GitHub reports their
// advisories, but nobody needs npm to build the website.
const lockfile = "website/package-lock.json"

// npmPackage is what the lockfile records for a package.
type npmPackage struct {
	Version   string `json:"version"`
	Resolved  string `json:"resolved"`
	Integrity string `json:"integrity"`
}

// readLockfile returns the packages of the lockfile at path, keyed by name.
func readLockfile(path string) (map[string]npmPackage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lock struct {
		Packages map[string]npmPackage `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	packages := make(map[string]npmPackage)
	for key, pkg := range lock.Packages {
		if name, ok := strings.CutPrefix(key, "node_modules/"); ok {
			packages[name] = pkg
		}
	}
	return packages, nil
}

// npmFile returns the file at path inside the package called name.
func npmFile(cacheDir string, packages map[string]npmPackage, name, path string) ([]byte, error) {
	pkg, ok := packages[name]
	if !ok {
		return nil, fmt.Errorf("%s pins no %s", lockfile, name)
	}
	tarball, err := download(cacheDir, pkg.Resolved, pkg.Integrity)
	if err != nil {
		return nil, err
	}
	data, err := extract(tarball, "package/"+path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pkg.Resolved, err)
	}
	return data, nil
}

// extract returns the file called name from a .tar.gz archive.
func extract(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("no %s in the archive", name)
		}
		if err != nil {
			return nil, err
		}
		if header.Name == name {
			return io.ReadAll(tr)
		}
	}
}
