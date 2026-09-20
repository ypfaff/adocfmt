package renderequivalence

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ypfaff/adocfmt/internal/format"
)

// casesDir is relative to this package.
const casesDir = "../../testdata/asciidoctor-cases"

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

	for _, path := range asciidoctorCases(t) {
		name := strings.TrimSuffix(strings.TrimPrefix(path, casesDir+"/"), ".adoc")
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

func asciidoctorCases(t *testing.T) []string {
	t.Helper()

	var cases []string
	err := filepath.WalkDir(casesDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".adoc" && entry.Name() != "README.adoc" {
			cases = append(cases, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading the cases: %v (run go run ./tools/fetch-asciidoctor-cases)", err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases found; run go run ./tools/fetch-asciidoctor-cases")
	}
	return cases
}
