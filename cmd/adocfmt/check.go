package main

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ypfaff/adocfmt/internal/format"
)

func checkAll(paths []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(paths) == 0 {
		src, err := io.ReadAll(stdin)
		if err != nil {
			return fail(stderr, "reading %s: %v", stdinName, err)
		}
		return check(src, stdinName, stdout, stderr)
	}
	return eachFile(paths, stderr, func(src []byte, path string) int {
		return check(src, path, stdout, stderr)
	})
}

// check names the document on stdout, the way gofmt -l does, so the list pipes
// into the command that acts on it.
func check(src []byte, name string, stdout, stderr io.Writer) int {
	out, err := format.Format(src)
	if err != nil {
		report(stderr, name, err)
		return exitError
	}
	if bytes.Equal(src, out) {
		return exitOK
	}
	_, _ = fmt.Fprintln(stdout, name)
	return exitChanged
}
