// Command adocfmt formats AsciiDoc source.
//
// Usage:
//
//	adocfmt [flags] [path ...]
//
// docs/reference/rules.adoc says what it changes.
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

// What the command exits with. The order is the precedence: an error outweighs
// a file that would change.
const (
	exitOK      = 0
	exitChanged = 1
	exitError   = 2
)

const stdinName = "<stdin>"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run takes the process streams as arguments, so every exit code is reachable
// from a test that starts no process.
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
	// path arrives here as a path rather than as an error.
	for _, path := range paths {
		if strings.HasPrefix(path, "-") {
			return fail(stderr, "%s stands behind a path, and flags come first", path)
		}
	}

	switch {
	case opts.write && opts.check:
		return fail(stderr, "--write and --check contradict each other")
	case opts.write && len(paths) == 0:
		return fail(stderr, "--write needs a path, and stdin is no file to write back to")
	}

	switch {
	case opts.check:
		return checkAll(paths, stdin, stdout, stderr)
	case opts.write:
		return writeAll(paths, stderr)
	default:
		return emitOne(paths, stdin, stdout, stderr)
	}
}

// fail puts the program name in front, because a reader of a CI log has to see
// who is speaking. A write to stderr that fails cannot be reported anywhere, so
// its error is dropped.
func fail(stderr io.Writer, message string, args ...any) int {
	_, _ = fmt.Fprintf(stderr, "adocfmt: "+message+"\n", args...)
	return exitError
}

// report writes path:line: message, the form an editor and an errorformat read.
// A source the scanner rejects outright carries no line, so it gets the name
// alone.
func report(stderr io.Writer, name string, err error) {
	var refusal *format.Refusal
	// errors.As matches a nil *Refusal, which carries no finding to place.
	if !errors.As(err, &refusal) || refusal == nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return
	}
	for _, finding := range refusal.Findings {
		_, _ = fmt.Fprintf(stderr, "%s:%d: %s\n", name, finding.Line, finding.Message)
	}
}

func release() string {
	if commit == "" {
		return version
	}
	return fmt.Sprintf("%s (%s, %s)", version, commit, date)
}
