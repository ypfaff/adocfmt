package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// extensions are what a walk picks up. A path named on the command line is
// formatted whatever its extension, so a hook that passes the files it staged
// reaches all of them.
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

// walk lists the AsciiDoc files below dir, in path order.
//
// It follows no symlink, which is what
// filepath.WalkDir does anyway: a link is no regular file, so the walk reaches
// neither the directory behind it nor the file.
func walk(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && slices.Contains(extensions, filepath.Ext(path)) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
