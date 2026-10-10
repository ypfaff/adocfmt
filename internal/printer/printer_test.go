package printer

import (
	"bytes"
	"testing"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/testdocs"
)

// TestPrintIsIdentity pins the one guarantee the tree gives: with every rule
// off, printing reproduces the source byte for byte. The render equivalence
// checks cannot stand in for this, because they collapse the whitespace a lost
// blank line would show up in.
func TestPrintIsIdentity(t *testing.T) {
	for _, doc := range testdocs.Documents(t) {
		t.Run(doc.Name, func(t *testing.T) {
			t.Parallel()

			tree, err := block.Parse(doc.Src)
			if err != nil {
				t.Fatal(err)
			}
			if got := PrintRaw(tree); !bytes.Equal(got, doc.Src) {
				t.Errorf("output differs from the source at byte %d", firstDifference(got, doc.Src))
			}
		})
	}
}

func firstDifference(a, b []byte) int {
	for at := range min(len(a), len(b)) {
		if a[at] != b[at] {
			return at
		}
	}
	return min(len(a), len(b))
}
