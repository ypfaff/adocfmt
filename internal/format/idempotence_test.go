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

// TestSecondRunAfterRubyWhitespace pins what FuzzFormatIsIdempotent found where
// a line ends in a character Asciidoctor's rstrip strips beyond spaces and
// tabs: \v, \f or NUL. The printer has to read the line as Asciidoctor does,
// or the first run writes what the second strips.
func TestSecondRunAfterRubyWhitespace(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "NUL as a document title", src: "= \x00 =\n", want: "= \x00 =\n"},
		{name: "NUL as a section title", src: "== \x00 ==\n", want: "== \x00 ==\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			once, err := Format([]byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			if string(once) != test.want {
				t.Errorf("got %q, want %q", once, test.want)
			}
			checkSecondRun(t, once)
		})
	}
}
