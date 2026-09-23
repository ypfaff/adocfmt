// Command adocfmt formats AsciiDoc source.
//
// Usage:
//
//	adocfmt [flags] [path ...]
//
// docs/rules.adoc says what it changes.
package main

import (
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

// What the command exits with.
const (
	exitOK    = 0
	exitError = 2
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

	switch len(paths) {
	case 0:
		src, err := io.ReadAll(stdin)
		if err != nil {
			return fail(stderr, "reading %s: %v", stdinName, err)
		}
		return emit(src, stdinName, stdout, stderr)
	case 1:
		src, err := os.ReadFile(paths[0])
		if err != nil {
			return fail(stderr, "%v", err)
		}
		return emit(src, paths[0], stdout, stderr)
	default:
		return fail(stderr, "more than one path, and nowhere to write the results")
	}
}

// emit writes the formatted document to stdout, which is what the command does
// when no flag selects a mode.
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
