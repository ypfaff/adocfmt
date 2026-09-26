package format

import (
	"bytes"
	"os"
	"testing"

	"github.com/ypfaff/adocfmt/internal/corpus"
)

// corpusDir is relative to this package.
const corpusDir = "../../" + corpus.Dir

// TestFormatIsIdempotent holds the guarantee that a second run has nothing
// left to do, over every case in the corpus. See docs/reference/rules.adoc.
//
// A refused document has no output to format a second time. Which cases those
// are is pinned in internal/renderequivalence, so one that newly refuses fails
// there instead of quietly dropping out of this test.
func TestFormatIsIdempotent(t *testing.T) {
	files, err := corpus.Files(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		t.Run(corpus.Name(corpusDir, path), func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			once, err := Format(src)
			if err != nil {
				t.Skipf("refused: %v", err)
			}
			twice, err := Format(once)
			if err != nil {
				t.Fatalf("output of the first run is refused: %v", err)
			}
			if !bytes.Equal(once, twice) {
				t.Errorf("the second run differs at %s", firstDifferingLine(once, twice))
			}
		})
	}
}
