package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// pagefind writes the Pagefind binary for this platform into cacheDir and
// returns its path. It is written anew on every build, so a half-written copy
// never lingers.
func pagefind(cacheDir string, packages map[string]npmPackage) (string, error) {
	// Pagefind ships one package per platform, named like Go's platform
	// except for the processor.
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	name := fmt.Sprintf("@pagefind/%s-%s", runtime.GOOS, arch)
	binary := "pagefind_extended"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	data, err := npmFile(cacheDir, packages, name, "bin/"+binary)
	if err != nil {
		return "", fmt.Errorf("pagefind for %s/%s: %w", runtime.GOOS, runtime.GOARCH, err)
	}
	target := filepath.Join(cacheDir, binary)
	return target, os.WriteFile(target, data, 0o755)
}
