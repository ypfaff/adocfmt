package format

import (
	"testing"

	"github.com/ypfaff/adocfmt/internal/testdocs"
)

// FuzzFormatIsIdempotent searches for source whose formatted form a second run
// refuses or changes. The seeds are the documents TestFormatIsIdempotent reads.
//
//	go test -fuzz=FuzzFormatIsIdempotent ./internal/format
func FuzzFormatIsIdempotent(f *testing.F) {
	for _, doc := range testdocs.Documents(f) {
		f.Add(doc.Src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		once, err := Format(src)
		if err != nil {
			return
		}
		checkSecondRun(t, once)
	})
}
