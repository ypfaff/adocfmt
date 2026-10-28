package renderequivalence

import (
	"os/exec"
	"testing"

	"github.com/ypfaff/adocfmt/internal/format"
	"github.com/ypfaff/adocfmt/internal/testdocs"
)

// notRenderable names the cases Asciidoctor cannot render here, and why.
// Everything else has to render, so a broken installation fails the run
// instead of passing with nothing compared.
var notRenderable = map[string]string{
	"syntax_highlighter/0043-should-honor-cgi-style-options-on-language-if-rouge-version-": "Asciidoctor 2.0.26 calls CGI.parse, which Ruby 4 removed",
}

// refused names the cases Format turns down, and why. They are documents
// Asciidoctor warns about, see decision record 0003. A case that formats
// after all fails, so the list tracks the parser.
var refused = byCase(map[string][]string{
	"a block never closes": {
		"attributes/0028-should-warn-if-unterminated-block-comment-is-detected-in-doc",
		"blocks/0010-should-warn-if-unterminated-comment-block-is-detected-in-bod",
		"blocks/0011-should-warn-if-unterminated-comment-block-is-detected-inside",
		"blocks/0056-should-warn-if-example-block-is-not-terminated",
		"blocks/0061-should-warn-if-listing-block-is-not-terminated",
		"lists/0060-should-warn-if-unterminated-block-is-detected-in-list-item",
		"sections/0024-should-not-recognize-section-title-that-does-not-contain-alp",
		"sections/0025-should-not-recognize-section-title-that-consists-of-only-und",
		"tables/0099-should-warn-if-table-block-is-not-terminated",
	},
	"a conditional region never closes": {
		"reader/0100-should-log-error-with-end-position-if-preprocessor-condition",
		"reader/0101-should-log-error-with-start-location-if-preprocessor-conditi",
		"reader/0102-should-log-error-if-multiple-preprocessor-conditional-direct",
	},
	"an endif closes nothing": {
		"reader/0078-should-log-warning-if-endif-is-unmatched",
	},
	"a directive is malformed": {
		"reader/0080-should-log-warning-if-endif-contains-text",
		"reader/0094-should-warn-if-ifeval-has-target",
		"reader/0095-should-warn-if-ifeval-has-invalid-expression",
		"reader/0096-should-warn-if-ifeval-is-missing-expression",
		"reader/0097-ifdef-with-no-target-is-ignored",
		"reader/0098-should-not-warn-about-invalid-ifdef-preprocessor-directive-i",
		"reader/0099-should-not-warn-about-invalid-ifeval-preprocessor-directive-",
	},
})

func byCase(byCause map[string][]string) map[string]string {
	cases := map[string]string{}
	for cause, names := range byCause {
		for _, name := range names {
			cases[name] = cause
		}
	}
	return cases
}

func TestFormatIsRenderEquivalent(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("renders every document with Asciidoctor")
	}
	if _, err := exec.LookPath("ruby"); err != nil {
		t.Fatal("ruby is not on the PATH")
	}

	for _, doc := range testdocs.Documents(t) {
		t.Run(doc.Name, func(t *testing.T) {
			t.Parallel()

			if reason, ok := notRenderable[doc.Name]; ok {
				t.Skip(reason)
			}

			findings, err := Differences(doc.Src, formatOrSkip(t, doc))
			if err != nil {
				t.Fatal(err)
			}
			for _, finding := range findings {
				t.Error(finding)
			}
		})
	}
}

func formatOrSkip(t *testing.T, doc testdocs.Document) []byte {
	t.Helper()

	formatted, err := format.Format(doc.Src)
	cause, listed := refused[doc.Name]
	switch {
	case listed && err == nil:
		t.Fatal("formats; remove it from refused")
	case listed:
		t.Skipf("%s: %v", cause, err)
	case err != nil:
		t.Fatal(err)
	}
	return formatted
}
