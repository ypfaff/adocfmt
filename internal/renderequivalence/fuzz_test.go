package renderequivalence

import (
	"os/exec"
	"testing"

	"github.com/ypfaff/adocfmt/internal/format"
	"github.com/ypfaff/adocfmt/internal/testdocs"
)

// FuzzFormatIsRenderEquivalent searches for source whose formatted form
// Asciidoctor renders differently. The seeds are the documents
// TestFormatIsRenderEquivalent reads.
//
//	go test -fuzz=FuzzFormatIsRenderEquivalent ./internal/renderequivalence
func FuzzFormatIsRenderEquivalent(f *testing.F) {
	if testing.Short() {
		f.Skip("renders every input with Asciidoctor")
	}
	if _, err := exec.LookPath("ruby"); err != nil {
		f.Fatal("ruby is not on the PATH")
	}

	for _, doc := range testdocs.Documents(f) {
		f.Add(doc.Src)
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
