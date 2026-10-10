package renderequivalence

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ypfaff/adocfmt/internal/corpus"
	"github.com/ypfaff/adocfmt/internal/format"
	"github.com/ypfaff/adocfmt/internal/golden"
)

// goldenDir is relative to this package.
const goldenDir = "../../" + golden.Dir

// FuzzFormatIsRenderEquivalent searches for source whose formatted form
// Asciidoctor renders differently. The seeds are every Asciidoctor case and
// every golden input.
//
//	go test -fuzz=FuzzFormatIsRenderEquivalent ./internal/renderequivalence
func FuzzFormatIsRenderEquivalent(f *testing.F) {
	if testing.Short() {
		f.Skip("renders every input with Asciidoctor")
	}
	if _, err := exec.LookPath("ruby"); err != nil {
		f.Fatal("ruby is not on the PATH")
	}

	files, err := corpus.Files(casesDir)
	if err != nil {
		f.Fatal(err)
	}
	cases, err := golden.Cases(goldenDir)
	if err != nil {
		f.Fatal(err)
	}
	for _, dir := range cases {
		files = append(files, filepath.Join(dir, golden.InputFile))
	}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		formatted, err := format.Format(src)
		if err != nil {
			return
		}
		findings, err := Differences(src, formatted)
		if err != nil {
			// An input that Asciidoctor itself fails on says nothing about
			// the formatter.
			if _, srcErr := render(src); srcErr != nil {
				return
			}
			t.Fatal(err)
		}
		for _, finding := range findings {
			t.Error(finding)
		}
	})
}
