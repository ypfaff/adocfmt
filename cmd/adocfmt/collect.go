package main

import (
	"io"
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
// in the order they were named, and returns the exit code the reading earned.
// A path it cannot read is reported and passed over, the way a file that cannot
// be formatted is, so one bad path does not hide the rest of the run.
//
// os.Stat follows a symlink, so a link named on the command line is collected
// like the file it points at.
func collect(paths []string, stderr io.Writer) ([]string, int) {
	var files []string
	code := exitOK
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			code = max(code, fail(stderr, "%v", err))
			continue
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		found, walked := walk(path, stderr)
		files = append(files, found...)
		code = max(code, walked)
	}
	return files, code
}

// walk lists the AsciiDoc files below dir, in path order and under dir's own
// name. A link inside the tree is passed over, which is what filepath.WalkDir
// does anyway: a link is no regular file, so the walk reaches neither the
// directory behind it nor the file.
func walk(dir string, stderr io.Writer) ([]string, int) {
	// WalkDir Lstats its root, so a linked directory has to be read through
	// what it resolves to.
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fail(stderr, "%s: %v", dir, err)
	}

	var files []string
	code := exitOK
	// The callback answers every error itself and returns fs.SkipDir alone,
	// which the walk does not hand back.
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// The error names the resolved path; dir is the one its reader
			// typed. What lies behind it is unreachable, the rest is not.
			code = max(code, fail(stderr, "%s: %v", dir, err))
			return fs.SkipDir
		}
		if entry.Type().IsRegular() && hasExtension(path) {
			files = append(files, filepath.Join(dir, strings.TrimPrefix(path, root)))
		}
		return nil
	})
	return files, code
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
