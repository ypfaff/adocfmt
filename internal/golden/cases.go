// Package golden locates the golden file cases, which the tests compare against
// and tools/update-golden rewrites.
//
// See docs/contributing/testing-strategy.adoc for what a case is and when to
// add one.
package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Dir holds the cases, relative to the repository root.
const Dir = "testdata/golden"

// The two files every case is made of.
const (
	InputFile  = "input.adoc"
	GoldenFile = "golden.adoc"
)

// Cases returns the directory of every case below root, in path order. A
// directory holding only one of the two files still counts, so a typo surfaces
// instead of dropping the case silently.
func Cases(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || (entry.Name() != InputFile && entry.Name() != GoldenFile) {
			return nil
		}
		if dir := filepath.Dir(path); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the cases: %w", err)
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no cases in %s", root)
	}
	return dirs, nil
}
