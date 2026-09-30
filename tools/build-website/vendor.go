package main

import (
	"os"
	"path"
	"path/filepath"
)

// vendorFiles are the scripts the pages load from vendor/: a package of the
// lockfile and the path of the script inside it.
var vendorFiles = []struct{ pkg, path string }{
	{"@highlightjs/cdn-assets", "highlight.min.js"},
	{"mermaid", "dist/mermaid.min.js"},
}

// vendor writes the vendor files into the vendor directory of site.
func vendor(cacheDir string, packages map[string]npmPackage, site string) error {
	dir := filepath.Join(site, "vendor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, file := range vendorFiles {
		data, err := npmFile(cacheDir, packages, file.pkg, file.path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, path.Base(file.path)), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
