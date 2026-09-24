package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// tree builds a directory holding one file of every shape the walk has to tell
// apart, and returns it.
func tree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{".hidden", "nested"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		".hidden/deep.adoc",
		"a.adoc",
		"b.asciidoc",
		"nested/c.adoc",
		"notes.txt",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("Text.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// collected runs collect and fails the test if it reported anything, so a case
// about what the walk picks up says nothing about what it cannot read.
func collected(t *testing.T, paths ...string) []string {
	t.Helper()

	var stderr bytes.Buffer
	files, code := collect(paths, &stderr)
	if code != exitOK {
		t.Fatalf("got exit %d, want %d (stderr: %s)", code, exitOK, &stderr)
	}
	return files
}

// TestCollectWalksADirectory pins what a walk picks up: both extensions, at
// every depth, and a directory whose name begins with a dot like any other,
// because a name to skip is a guess this command gives no way to take back.
func TestCollectWalksADirectory(t *testing.T) {
	t.Parallel()

	collectsTheWholeTree(t, tree(t))
}

// collectsTheWholeTree fails unless collect walks root into the four AsciiDoc
// files tree built, each under root's own name.
func collectsTheWholeTree(t *testing.T, root string) {
	t.Helper()

	files := collected(t, root)
	want := []string{
		filepath.Join(root, ".hidden", "deep.adoc"),
		filepath.Join(root, "a.adoc"),
		filepath.Join(root, "b.asciidoc"),
		filepath.Join(root, "nested", "c.adoc"),
	}
	if !slices.Equal(files, want) {
		t.Errorf("got %v, want %v", files, want)
	}
}

// TestCollectTakesANamedFileWhateverItsEnding pins that the extensions gate the
// walk alone, so a hook passing the files it staged reaches all of them.
func TestCollectTakesANamedFileWhateverItsEnding(t *testing.T) {
	t.Parallel()

	root := tree(t)
	notes := filepath.Join(root, "notes.txt")
	files := collected(t, notes)
	if !slices.Equal(files, []string{notes}) {
		t.Errorf("got %v, want %v", files, []string{notes})
	}
}

// TestCollectWalksAnExtensionInAnyCase pins that case does not decide what the
// walk picks up, because a path named on the command line is taken whatever it
// is called and the two entry points have to agree.
func TestCollectWalksAnExtensionInAnyCase(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"LOUD.ADOC", "Mixed.AsciiDoc"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("Text.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files := collected(t, root)
	want := []string{
		filepath.Join(root, "LOUD.ADOC"),
		filepath.Join(root, "Mixed.AsciiDoc"),
	}
	if !slices.Equal(files, want) {
		t.Errorf("got %v, want %v", files, want)
	}
}

// TestCollectAndSymlinks pins both halves of the rule: a link named on the
// command line is a file like any other, and one the walk runs into is passed
// over, so the file behind it is not formatted twice.
func TestCollectAndSymlinks(t *testing.T) {
	t.Parallel()

	root := tree(t)
	target := filepath.Join(root, "a.adoc")
	link := filepath.Join(root, "link.adoc")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this file system has no symlinks: %v", err)
	}

	named := collected(t, link)
	if !slices.Equal(named, []string{link}) {
		t.Errorf("a named link collected %v, want %v", named, []string{link})
	}

	walked := collected(t, root)
	if slices.Contains(walked, link) {
		t.Errorf("the walk collected the link %s", link)
	}
}

// TestCollectWalksALinkedDirectory pins that a link to a directory is walked
// like the directory it stands for, and that the files come back under the name
// the command line used. The walk would otherwise end at the link and report
// nothing at all.
func TestCollectWalksALinkedDirectory(t *testing.T) {
	t.Parallel()

	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(tree(t), link); err != nil {
		t.Skipf("this file system has no symlinks: %v", err)
	}

	collectsTheWholeTree(t, link)
}

// TestCollectReportsAPathItCannotReadAndGoesOn pins that one unreadable path
// costs its own files alone: the ones named around it are still collected, and
// the error decides the exit code.
func TestCollectReportsAPathItCannotReadAndGoesOn(t *testing.T) {
	t.Parallel()

	root := tree(t)
	a := filepath.Join(root, "a.adoc")
	b := filepath.Join(root, "b.asciidoc")
	missing := filepath.Join(root, "missing.adoc")

	var stderr bytes.Buffer
	files, code := collect([]string{a, missing, b}, &stderr)

	if want := []string{a, b}; !slices.Equal(files, want) {
		t.Errorf("got %v, want %v", files, want)
	}
	if code != exitError {
		t.Errorf("got exit %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), missing) {
		t.Errorf("stderr is %q, want it to name %s", &stderr, missing)
	}
}

// TestCollectReportsADirectoryItCannotRead pins the same for a subdirectory the
// walk is shut out of: it costs what lies below it, not the tree around it.
func TestCollectReportsADirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	root := tree(t)
	shut := filepath.Join(root, "nested")
	if err := os.Chmod(shut, 0o000); err != nil {
		t.Fatal(err)
	}
	// t.TempDir removes the tree afterwards, which it cannot do through this.
	t.Cleanup(func() { _ = os.Chmod(shut, 0o700) })
	if _, err := os.ReadDir(shut); err == nil {
		t.Skip("this user reads a directory whatever its mode")
	}

	var stderr bytes.Buffer
	files, code := collect([]string{root}, &stderr)

	want := []string{
		filepath.Join(root, ".hidden", "deep.adoc"),
		filepath.Join(root, "a.adoc"),
		filepath.Join(root, "b.asciidoc"),
	}
	if !slices.Equal(files, want) {
		t.Errorf("got %v, want %v", files, want)
	}
	if code != exitError {
		t.Errorf("got exit %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "nested") {
		t.Errorf("stderr is %q, want it to name the directory", &stderr)
	}
}
