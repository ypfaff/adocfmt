package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// extensions are what a directory walk picks up.
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
			// A path named on the command line is taken whatever its extension.
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
// name. A link inside the tree is passed over, which is what fs.WalkDir does
// anyway: a link is no regular file, so the walk reaches neither the directory
// behind it nor the file.
func walk(dir string, stderr io.Writer) ([]string, int) {
	var files []string
	code := exitOK
	// The callback returns no error but fs.SkipDir, and WalkDir does not pass
	// that one on, so its result is always nil.
	_ = fs.WalkDir(os.DirFS(dir), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A directory that cannot be read costs only its own files, so
			// report it and walk on.
			code = max(code, fail(stderr, "%s: %v", dir, err))
			return fs.SkipDir
		}
		if entry.Type().IsRegular() && hasExtension(path) {
			files = append(files, filepath.Join(dir, filepath.FromSlash(path)))
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

// eachFile hands the source of every file below paths to do, and answers with
// the strongest exit code the run earned. A file it cannot read is reported and
// passed over, the way collect passes over a path it cannot read.
func eachFile(paths []string, stderr io.Writer, do func(src []byte, path string) int) int {
	files, code := collect(paths, stderr)
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			code = max(code, fail(stderr, "%v", err))
			continue
		}
		code = max(code, do(src, path))
	}
	return code
}
