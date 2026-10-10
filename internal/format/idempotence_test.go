package format

import (
	"bytes"
	"testing"

	"github.com/ypfaff/adocfmt/internal/testdocs"
)

// TestFormatIsIdempotent holds the guarantee that a second run has nothing
// left to do. See docs/reference/rules.adoc.
//
// A refused document has no output to format a second time. Which Asciidoctor
// cases those are is pinned in internal/renderequivalence, so one that newly
// refuses fails there instead of quietly dropping out of this test.
func TestFormatIsIdempotent(t *testing.T) {
	for _, doc := range testdocs.Documents(t) {
		t.Run(doc.Name, func(t *testing.T) {
			t.Parallel()

			once, err := Format(doc.Src)
			if err != nil {
				t.Skipf("refused: %v", err)
			}
			checkSecondRun(t, once)
		})
	}
}

// checkSecondRun formats the output of a first run again and fails if the
// second run refuses it or changes it.
func checkSecondRun(t *testing.T, once []byte) {
	t.Helper()

	twice, err := Format(once)
	if err != nil {
		t.Fatalf("output of the first run is refused: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Errorf("the second run differs at %s", firstDifferingLine(once, twice))
	}
}
