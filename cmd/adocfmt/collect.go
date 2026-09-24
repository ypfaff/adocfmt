package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// A path named on the command line is formatted whatever its extension, so a
// hook that passes the files it staged reaches all of them.
var extensions = []string{".adoc", ".asciidoc"}

// collect turns the paths named on the command line into the files to format,
// in the order they were named.
//
// os.Stat follows a symlink, so a link named on the command line is collected
// like the file it points at.
func collect(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		found, err := walk(path)
		if err != nil {
			return nil, err
		}
		files = append(files, found...)
	}
	return files, nil
}

// walk lists the AsciiDoc files below dir, in path order and under dir's own
// name. A link inside the tree is passed over, which is what filepath.WalkDir
// does anyway: a link is no regular file, so the walk reaches neither the
// directory behind it nor the file.
func walk(dir string) ([]string, error) {
	// WalkDir Lstats its root, so a linked directory has to be read through
	// what it resolves to.
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && hasExtension(path) {
			files = append(files, filepath.Join(dir, strings.TrimPrefix(path, root)))
		}
		return nil
	})
	return files, err
}

// hasExtension reports whether path carries one of the extensions. Case does
// not decide: UPPER.ADOC is an AsciiDoc file like any other, and the file
// system it lies on may not even tell the two spellings apart.
func hasExtension(path string) bool {
	ext := filepath.Ext(path)
	return slices.ContainsFunc(extensions, func(want string) bool {
		return strings.EqualFold(want, ext)
	})
}
