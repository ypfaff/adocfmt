package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()

		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	unformatted := write("unformatted.adoc", "Text.   \n\n\n== Section ==\n")
	refused := write("refused.adoc", "ifdef::extra[]\n\n----\ncode\n")
	mixed := write("mixed.adoc", "a\r\nb\n")
	missing := filepath.Join(dir, "missing.adoc")

	tests := []struct {
		name   string
		args   []string
		stdin  string
		code   int
		stdout string
		stderr string
	}{
		{
			name:   "stdin is formatted to stdout",
			stdin:  "Text.   \n\n\n== Section ==\n",
			code:   exitOK,
			stdout: "Text.\n\n== Section\n",
		},
		{
			name:   "one file is formatted to stdout and stays as it is",
			args:   []string{unformatted},
			code:   exitOK,
			stdout: "Text.\n\n== Section\n",
		},
		{
			name:   "a refused document names its file and line per finding",
			args:   []string{refused},
			code:   exitError,
			stderr: fmt.Sprintf("%[1]s:1: conditional region has no endif\n%[1]s:3: block has no closing delimiter\n", refused),
		},
		{
			name:   "a rejected source carries no line, so it gets the name alone",
			args:   []string{mixed},
			code:   exitError,
			stderr: fmt.Sprintf("%s: source has mixed line endings: 1 CRLF, 1 LF\n", mixed),
		},
		{
			name:   "a file that cannot be read is an error",
			args:   []string{missing},
			code:   exitError,
			stderr: "no such file or directory",
		},
		{
			name:   "more than one path has nowhere to put the results",
			args:   []string{unformatted, unformatted},
			code:   exitError,
			stderr: "more than one path",
		},
		{
			name:   "a flag behind a path is a mistake, not a file name",
			args:   []string{unformatted, "--write"},
			code:   exitError,
			stderr: "--write stands behind a path",
		},
		{
			name:   "an unknown flag is a complaint",
			args:   []string{"--nope"},
			code:   exitError,
			stderr: "not defined",
		},
		{
			name:   "help was asked for, so it is output",
			args:   []string{"--help"},
			code:   exitOK,
			stdout: "Usage:\n  adocfmt [flags] [path ...]",
		},
		{
			name:   "one dash reads the same as two",
			args:   []string{"-help"},
			code:   exitOK,
			stdout: "Usage:\n  adocfmt [flags] [path ...]",
		},
		{
			name:   "the version is output too",
			args:   []string{"--version"},
			code:   exitOK,
			stdout: "adocfmt ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(test.args, strings.NewReader(test.stdin), &stdout, &stderr)

			if code != test.code {
				t.Errorf("got exit %d, want %d (stderr: %s)", code, test.code, &stderr)
			}
			if !strings.Contains(stdout.String(), test.stdout) {
				t.Errorf("stdout is %q, want it to hold %q", &stdout, test.stdout)
			}
			if !strings.Contains(stderr.String(), test.stderr) {
				t.Errorf("stderr is %q, want it to hold %q", &stderr, test.stderr)
			}

			// The streams are the command's promise: what was asked for goes to
			// stdout, a complaint to stderr, and neither run carries both.
			if code == exitOK && stderr.Len() > 0 {
				t.Errorf("a clean run wrote to stderr: %s", &stderr)
			}
			if code != exitOK && stdout.Len() > 0 {
				t.Errorf("a failed run wrote to stdout: %s", &stdout)
			}
		})
	}
}

// TestRunLeavesTheFileAlone pins that no mode flag means no write, so the file
// the command read comes out of the run as it went in.
func TestRunLeavesTheFileAlone(t *testing.T) {
	t.Parallel()

	src := "Text.   \n\n\n== Section ==\n"
	path := filepath.Join(t.TempDir(), "doc.adoc")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitOK, &stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != src {
		t.Errorf("the file changed to %q, want %q", after, src)
	}
}

// TestUsageNamesEveryFlag guards the one thing a handwritten usage can get
// wrong that a generated one cannot.
func TestUsageNamesEveryFlag(t *testing.T) {
	t.Parallel()

	var text bytes.Buffer
	usage(&text)
	for _, entry := range (&options{}).entries() {
		if !strings.Contains(text.String(), "--"+entry.long) {
			t.Errorf("the usage does not name --%s", entry.long)
		}
		if entry.short != "" && !strings.Contains(text.String(), "-"+entry.short+",") {
			t.Errorf("the usage does not name -%s", entry.short)
		}
	}
}

// TestRelease pins that a stamped build names its tag and an unstamped one says
// it is not one. It writes package state, so it does not run in parallel.
func TestRelease(t *testing.T) {
	if got, want := release(), "(devel)"; got != want {
		t.Errorf("an unstamped build reports %q, want %q", got, want)
	}

	version, commit, date = "v1.2.3", "abc1234", "2026-09-23"
	t.Cleanup(func() { version, commit, date = "(devel)", "", "" })

	if got, want := release(), "v1.2.3 (abc1234, 2026-09-23)"; got != want {
		t.Errorf("a stamped build reports %q, want %q", got, want)
	}
}
