package printer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/corpus"
	"github.com/ypfaff/adocfmt/internal/golden"
)

// The two directories are relative to this package.
const (
	corpusDir = "../../" + corpus.Dir
	goldenDir = "../../" + golden.Dir
)

// TestPrintIsIdentity pins the one guarantee the tree gives: with every rule
// off, printing reproduces the source byte for byte. The render equivalence
// checks cannot stand in for this, because they collapse the whitespace a lost
// blank line would show up in.
func TestPrintIsIdentity(t *testing.T) {
	for name, path := range documents(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := block.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			if got := PrintRaw(doc); !bytes.Equal(got, src) {
				t.Errorf("output differs from the source at byte %d", firstDifference(got, src))
			}
		})
	}
}

// documents lists every AsciiDoc file the tests own, by name: the Asciidoctor
// cases and both sides of every golden case.
func documents(t *testing.T) map[string]string {
	t.Helper()

	docs := map[string]string{}
	files, err := corpus.Files(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		docs[corpus.Name(corpusDir, path)] = path
	}

	cases, err := golden.Cases(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range cases {
		for _, file := range []string{golden.InputFile, golden.GoldenFile} {
			path := filepath.Join(dir, file)
			docs[corpus.Name(goldenDir, path)] = path
		}
	}
	return docs
}

func firstDifference(a, b []byte) int {
	for at := range min(len(a), len(b)) {
		if a[at] != b[at] {
			return at
		}
	}
	return min(len(a), len(b))
}
