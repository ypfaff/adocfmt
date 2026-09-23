// Command adocfmt formats AsciiDoc source.
//
// Usage:
//
//	adocfmt [flags] [path ...]
//
// docs/rules.adoc says what it changes.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ypfaff/adocfmt/internal/format"
)

// What the release pipeline stamps in with -ldflags -X
var (
	version = "(development)"
	commit  string
	date    string
)

// What the command exits with. The order is the precedence: a run that both
// fails on one file and would change another reports the failure.
const (
	exitOK      = 0
	exitChanged = 1
	exitError   = 2
)

// stdinName is what a document read from stdin is called in a diagnostic.
const stdinName = "<stdin>"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the whole command. main only hands it the process streams, so every
// exit code is reachable from a test that starts no process.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var opts options
	flags := opts.register()
	if err := flags.Parse(args); err != nil {
		return fail(stderr, "%v; run adocfmt --help for the flags", err)
	}
	switch {
	case opts.help:
		usage(stdout)
		return exitOK
	case opts.version:
		_, _ = fmt.Fprintf(stdout, "adocfmt %s\n", release())
		return exitOK
	}

	paths := flags.Args()
	// flag stops at the first argument that is no flag, so one written behind a
	// path arrives here as a path. Without this it would be reported as a file
	// that does not exist.
	for _, path := range paths {
		if strings.HasPrefix(path, "-") {
			return fail(stderr, "%s stands behind a path, and flags come first", path)
		}
	}

	if opts.check {
		return checkAll(paths, stdin, stdout, stderr)
	}
	return emitOne(paths, stdin, stdout, stderr)
}

// checkAll names every document formatting would change and writes none of
// them, so a pipeline is gated without a diff.
func checkAll(paths []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(paths) == 0 {
		src, err := io.ReadAll(stdin)
		if err != nil {
			return fail(stderr, "reading %s: %v", stdinName, err)
		}
		return check(src, stdinName, stdout, stderr)
	}

	files, err := collect(paths)
	if err != nil {
		return fail(stderr, "%v", err)
	}
	code := exitOK
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			code = max(code, fail(stderr, "%v", err))
			continue
		}
		code = max(code, check(src, path, stdout, stderr))
	}
	return code
}

// emitOne writes one formatted document to stdout, which is what the command
// does when no flag selects a mode. It is one, because a directory or a second
// path would fill a terminal with documents nobody asked to read.
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
			return fail(stderr, "%s is a directory; use --check to walk it", paths[0])
		}
		src, err := os.ReadFile(paths[0])
		if err != nil {
			return fail(stderr, "%v", err)
		}
		return emit(src, paths[0], stdout, stderr)
	default:
		return fail(stderr, "more than one path; use --check to read them all")
	}
}

// check names the document on stdout when formatting would change it, the way
// gofmt -l does, so the list pipes into the command that acts on it.
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

// emit writes the formatted document to stdout. The write is checked, because
// this one carries the product: a document cut short by a full disk must not
// leave the run reporting success.
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

// fail writes a complaint about the run and hands back the code to exit with.
// The program name goes in front, because a reader of a CI log has to see who
// is speaking.
//
// A write to stderr that fails cannot be reported anywhere, so its error is
// dropped here rather than at each of the callers.
func fail(stderr io.Writer, message string, args ...any) int {
	_, _ = fmt.Fprintf(stderr, "adocfmt: "+message+"\n", args...)
	return exitError
}

// report names the file in front of every finding, in the form an editor and an
// errorformat read. A source the scanner rejects outright carries no line, so it
// gets the name alone.
func report(stderr io.Writer, name string, err error) {
	var refusal *format.Refusal
	if !errors.As(err, &refusal) {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return
	}
	for _, finding := range refusal.Findings {
		_, _ = fmt.Fprintf(stderr, "%s:%d: %s\n", name, finding.Line, finding.Message)
	}
}

// release is the line --version prints.
func release() string {
	if commit == "" {
		return version
	}
	return fmt.Sprintf("%s (%s, %s)", version, commit, date)
}
