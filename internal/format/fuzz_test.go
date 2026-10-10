package format

import (
	"os"
	"testing"

	"github.com/ypfaff/adocfmt/internal/asciidoctorcases"
	"github.com/ypfaff/adocfmt/internal/golden"
)

// FuzzFormatIsIdempotent searches for source whose formatted form a second run
// refuses or changes. The seeds are every Asciidoctor case and every golden
// input.
//
//	go test -fuzz=FuzzFormatIsIdempotent ./internal/format
func FuzzFormatIsIdempotent(f *testing.F) {
	files, err := asciidoctorcases.Files(asciidoctorCasesDir)
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(src)
	}
	for _, dir := range goldenCases(f) {
		f.Add(readCase(f, dir, golden.InputFile))
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		once, err := Format(src)
		if err != nil {
			return
		}
		checkSecondRun(t, once)
	})
}
