package renderequivalence

import (
	"slices"
	"strings"
	"testing"
)

func TestNormalizeHTML(t *testing.T) {
	if got, want := normalizeHTML("<p>a\n  b</p>\n"), "<p>a b</p>"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if normalizeHTML("<p>a</p>") == normalizeHTML("<p>b</p>") {
		t.Error("collapsing whitespace hid a difference in the text")
	}
}

func TestVerbatimBlocks(t *testing.T) {
	got := verbatimBlocks(`<pre class="highlight">a &lt; b</pre><p>x</p><pre>  c</pre>`)
	if want := []string{"a < b", "  c"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComments(t *testing.T) {
	src := strings.Join([]string{"// line", "text", "////", "block", "////", "  // indented"}, "\n")
	got := comments([]byte(src))
	if want := []string{"// line", "////", "////", "// indented"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestChecksCatchCorruption proves the checks can fail.
//
// Format returns its input unchanged for now, so every case compares a document
// against itself and passes whether the checks work or not. This test
// hands them output that differs in a known way instead, and asserts that the
// check meant to catch that difference is the one that reports it.
func TestChecksCatchCorruption(t *testing.T) {
	const src = "= Title\n\n// a comment\nSome prose.\n\n----\nkeep   these   spaces\n----\n"

	tests := []struct {
		name    string
		corrupt string
		want    string
	}{
		{"changed prose", strings.Replace(src, "Some prose.", "Other prose.", 1), "rendering differs"},
		{"dropped comment", strings.Replace(src, "// a comment\n", "", 1), "comments differ"},
		// Whitespace inside a listing survives normalizeHTML, so only the
		// byte-exact verbatim check can see this one.
		{"squeezed verbatim", strings.Replace(src, "keep   these", "keep these", 1), "verbatim content differs"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			findings, err := Differences([]byte(src), []byte(test.corrupt))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(findings, func(f string) bool { return strings.HasPrefix(f, test.want) }) {
				t.Errorf("want a %q finding, got %v", test.want, findings)
			}
		})
	}
}
