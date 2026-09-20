package renderequivalence

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ypfaff/adocfmt/internal/corpus"
	"github.com/ypfaff/adocfmt/internal/format"
)

// casesDir is relative to this package.
const casesDir = "../../" + corpus.Dir

// notRenderable names the cases Asciidoctor cannot render here, and why.
// Everything else has to render, so a broken installation fails the run
// instead of passing with nothing compared.
var notRenderable = map[string]string{
	"syntax_highlighter/0043-should-honor-cgi-style-options-on-language-if-rouge-version-": "Asciidoctor 2.0.26 calls CGI.parse, which Ruby 4 removed",
}

func TestAsciidoctorCases(t *testing.T) {
	if testing.Short() {
		t.Skip("rendering every case takes about half a minute")
	}
	if _, err := exec.LookPath("asciidoctor"); err != nil {
		t.Fatal("asciidoctor is not on the PATH")
	}

	files, err := corpus.Files(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		name := corpus.Name(casesDir, path)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if reason, ok := notRenderable[name]; ok {
				t.Skip(reason)
			}

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			formatted, err := format.Format(src)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := Differences(src, formatted)
			if err != nil {
				t.Fatal(err)
			}
			for _, finding := range findings {
				t.Error(finding)
			}
		})
	}
}
