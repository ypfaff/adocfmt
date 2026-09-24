package main

import (
	"os"
	"path/filepath"
	"slices"
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

	files, err := collect([]string{root})
	if err != nil {
		t.Fatal(err)
	}
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
	files, err := collect([]string{notes})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, []string{notes}) {
		t.Errorf("got %v, want %v", files, []string{notes})
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

	named, err := collect([]string{link})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(named, []string{link}) {
		t.Errorf("a named link collected %v, want %v", named, []string{link})
	}

	walked, err := collect([]string{root})
	if err != nil {
		t.Fatal(err)
	}
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

// TestCollectReportsAPathThatIsNotThere pins that a path the command cannot
// read fails the run rather than being walked past.
func TestCollectReportsAPathThatIsNotThere(t *testing.T) {
	t.Parallel()

	if _, err := collect([]string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Error("got no error, want one")
	}
}
