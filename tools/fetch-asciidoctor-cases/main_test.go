package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ypfaff/adocfmt/internal/corpus"
)

const casesDir = "../../" + corpus.Dir

// TestReadmeMatchesCases fails when corpus.Version changes but the cases were
// not regenerated. CI installs the Asciidoctor that corpus.Version names, so
// without this check the old cases would quietly run against the new release.
func TestReadmeMatchesCases(t *testing.T) {
	files, err := corpus.Files(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(casesDir, "README.adoc"))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != readme(len(files)) {
		t.Errorf("the cases are not from Asciidoctor %s; run go run ./tools/fetch-asciidoctor-cases", asciidoctorTag)
	}
}
