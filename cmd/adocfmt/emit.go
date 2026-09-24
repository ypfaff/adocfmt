package main

import (
	"io"
	"os"

	"github.com/ypfaff/adocfmt/internal/format"
)

// emitOne writes one document, because a directory or a second path would fill
// a terminal with documents nobody asked to read.
func emitOne(paths []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch len(paths) {
	case 0:
		src, err := io.ReadAll(stdin)
		if err != nil {
			return fail(stderr, "reading %s: %v", stdinName, err)
		}
		return emit(src, stdinName, stdout, stderr)
	case 1:
		if info, err := os.Stat(paths[0]); err == nil && info.IsDir() {
			return fail(stderr, "%s is a directory; use --write or --check", paths[0])
		}
		src, err := os.ReadFile(paths[0])
		if err != nil {
			return fail(stderr, "%v", err)
		}
		return emit(src, paths[0], stdout, stderr)
	default:
		return fail(stderr, "more than one path; use --write or --check")
	}
}

// emit checks its write, because this one carries the product: a document cut
// short by a full disk must not report success.
func emit(src []byte, name string, stdout, stderr io.Writer) int {
	out, err := format.Format(src)
	if err != nil {
		report(stderr, name, err)
		return exitError
	}
	if _, err := stdout.Write(out); err != nil {
		return fail(stderr, "writing the result: %v", err)
	}
	return exitOK
}
