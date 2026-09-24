package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	unformattedSrc = "Text.   \n\n\n== Section ==\n"
	formattedSrc   = "Text.\n\n== Section\n"
)

// writeFile puts src in a file of its own and returns the path.
func writeFile(t *testing.T, dir, name, src string, mode fs.FileMode) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), mode); err != nil {
		t.Fatal(err)
	}
	// os.WriteFile applies the umask and sets no bit above the permissions.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteReplacesTheFile(t *testing.T) {
	t.Parallel()

	path := writeFile(t, t.TempDir(), "doc.adoc", unformattedSrc, 0o644)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--write", path}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitOK, &stderr)
	}
	if stdout.Len() > 0 {
		t.Errorf("stdout is %q, want nothing: the file is the result", &stdout)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != formattedSrc {
		t.Errorf("the file is %q, want %q", after, formattedSrc)
	}
}

// TestWriteKeepsTheMode pins that the file keeps the mode it had. The rename
// swaps the inode, so without carrying it over the file would come back at the
// 0600 os.CreateTemp gives a temporary file. The bits above the permissions go
// the same way, and a formatter that disarms a setuid script is worse than one
// that refuses to touch it.
func TestWriteKeepsTheMode(t *testing.T) {
	t.Parallel()

	for _, mode := range []fs.FileMode{
		0o644,
		0o755 | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, t.TempDir(), "doc.adoc", unformattedSrc, mode)

			var stdout, stderr bytes.Buffer
			if code := run([]string{"--write", path}, nil, &stdout, &stderr); code != exitOK {
				t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitOK, &stderr)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode(); got != mode {
				t.Errorf("the mode is %v, want %v", got, mode)
			}
		})
	}
}

// TestWriteTouchesNothingItDoesNotChange pins that a build system sees a new
// timestamp only where there is new content. An old modification time survives
// a run, because a rewrite would replace it with the time of the run.
func TestWriteTouchesNothingItDoesNotChange(t *testing.T) {
	t.Parallel()

	path := writeFile(t, t.TempDir(), "doc.adoc", formattedSrc, 0o644)
	old := time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--write", path}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitOK, &stderr)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("the file was rewritten: its time moved to %v", info.ModTime())
	}
}

// TestWriteThroughASymlinkKeepsTheLink pins that a link named on the command
// line still is one afterwards. The rename would otherwise put a regular file
// where the link stood and leave the document it pointed at unformatted.
func TestWriteThroughASymlinkKeepsTheLink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := writeFile(t, dir, "doc.adoc", unformattedSrc, 0o644)
	link := filepath.Join(dir, "link.adoc")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this file system has no symlinks: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--write", link}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitOK, &stderr)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Error("the link was replaced by a regular file")
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != formattedSrc {
		t.Errorf("the target is %q, want %q", after, formattedSrc)
	}
}

// TestWriteRefusesOneFileAndWritesTheRest pins that refusal is per file, and
// that the refusal still decides the exit code.
func TestWriteRefusesOneFileAndWritesTheRest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	good := writeFile(t, dir, "good.adoc", unformattedSrc, 0o644)
	bad := writeFile(t, dir, "bad.adoc", "ifdef::extra[]\n\n----\ncode\n", 0o644)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--write", dir}, nil, &stdout, &stderr); code != exitError {
		t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitError, &stderr)
	}

	written, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != formattedSrc {
		t.Errorf("the file beside the refused one is %q, want %q", written, formattedSrc)
	}
	left, err := os.ReadFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	if string(left) != "ifdef::extra[]\n\n----\ncode\n" {
		t.Errorf("the refused file was changed to %q", left)
	}
}
