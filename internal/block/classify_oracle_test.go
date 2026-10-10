package block

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ypfaff/adocfmt/internal/asciidoctorcases"
)

// asciidoctorCasesDir is relative to this package.
const asciidoctorCasesDir = "../../" + asciidoctorcases.Dir

// Each line of the Asciidoctor cases also runs edited, which carries it across the boundaries
// classify draws: a prefix that may turn it into metadata, a comment, an entry,
// a list item, a quote, an escaped or an indented line; after the first char,
// where a name starts, a space or a letter that Ruby counts and ASCII does not;
// a tab for the first space, and a space after the first ::, where a target
// starts; and at the end, each char that Ruby strips there.
var (
	prefixes = []string{".", "[", "[[", "//", "///", ":", "*", "> ", "\\", " "}
	inserts  = []string{" ", "é", "Ⓐ", "Ⅰ"}
)

// TestClassifyAgreesWithAsciidoctor reads every line as the first line of a
// block, once with classify and once with Asciidoctor, and fails on every line
// the two read differently. See docs/contributing/testing-strategy.adoc.
func TestClassifyAgreesWithAsciidoctor(t *testing.T) {
	if testing.Short() {
		t.Skip("reads every line with Asciidoctor")
	}
	if _, err := exec.LookPath("ruby"); err != nil {
		t.Fatal("ruby is not on the PATH")
	}

	lines, err := oracleLines()
	if err != nil {
		t.Fatal(err)
	}
	want, err := asciidoctorKinds(lines)
	if err != nil {
		t.Fatal(err)
	}

	type pair struct{ asciidoctor, classify string }
	disagree := map[pair][]string{}
	for i, l := range lines {
		src := []byte(l)
		ln := splitLines(src, 0)[0]
		// A lone + continues a list item only inside a list.
		if string(src[ln.text.Start:ln.text.End]) == "+" {
			continue
		}
		if got := kindName(classify(src, ln)); got != want[i] {
			p := pair{want[i], got}
			disagree[p] = append(disagree[p], l)
		}
	}
	pairs := slices.Collect(maps.Keys(disagree))
	slices.SortFunc(pairs, func(a, b pair) int {
		return cmp.Or(len(disagree[b])-len(disagree[a]), strings.Compare(a.asciidoctor+a.classify, b.asciidoctor+b.classify))
	})
	for _, p := range pairs {
		examples := disagree[p][:min(5, len(disagree[p]))]
		t.Errorf("%d lines Asciidoctor reads as %s and classify as %s, such as:\n%s",
			len(disagree[p]), p.asciidoctor, p.classify, quoteLines(examples))
	}
	t.Logf("%d lines read", len(lines))
}

// kindName names a shape the way line_kinds.rb names what Asciidoctor reads.
func kindName(sh shape) string {
	if sh.kind == shapeDirective && sh.bad {
		return "bad-directive"
	}
	return kindNames[sh.kind]
}

var kindNames = map[shapeKind]string{
	shapeText:         "text",
	shapeHeading:      "heading",
	shapeDelimiter:    "delimiter",
	shapeAttributes:   "attributes",
	shapeTitle:        "title",
	shapeAnchor:       "anchor",
	shapeComment:      "comment",
	shapeAttrEntry:    "attr-entry",
	shapeDirective:    "directive",
	shapeMarker:       "marker",
	shapeIndented:     "indented",
	shapeBreak:        "break",
	shapeMacro:        "macro",
	shapeQuote:        "quote",
	shapeContinuation: "continuation",
}

// oracleLines returns every distinct line of the Asciidoctor cases and its
// edits, in the order of the cases. They keep their trailing whitespace, so each side strips it
// itself. Blank lines are left out, since Asciidoctor skips them before it
// reads a block.
func oracleLines() ([]string, error) {
	files, err := asciidoctorcases.Files(asciidoctorCasesDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var lines []string
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, l := range Lines(src) {
			for _, edit := range edits(string(l)) {
				if strings.Trim(edit, rubySpace) != "" && !seen[edit] {
					seen[edit] = true
					lines = append(lines, edit)
				}
			}
		}
	}
	return lines, nil
}

func edits(l string) []string {
	out := []string{l, strings.Replace(l, " ", "\t", 1), strings.Replace(l, "::", ":: ", 1), l + " \t\v\f\r\x00"}
	for _, prefix := range prefixes {
		out = append(out, prefix+l)
	}
	if _, first := utf8.DecodeRuneInString(l); first > 0 {
		for _, insert := range inserts {
			out = append(out, l[:first]+insert+l[first:])
		}
	}
	return out
}

// asciidoctorKinds runs line_kinds.rb over the lines once and returns the
// kind it prints for each. It fails on an Asciidoctor other than the one the
// cases are pinned to, so an upgrade reruns the comparison on purpose.
func asciidoctorKinds(lines []string) ([]string, error) {
	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	for _, l := range lines {
		if err := enc.Encode(l); err != nil {
			return nil, err
		}
	}
	cmd := exec.Command("ruby", "testdata/line_kinds.rb")
	cmd.Stdin = &in
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("line_kinds.rb: %w: %s", err, stderr.String())
	}

	kinds := strings.Fields(string(out))
	if kinds[0] != asciidoctorcases.Version {
		return nil, fmt.Errorf("asciidoctor is %s, the cases are pinned to %s", kinds[0], asciidoctorcases.Version)
	}
	return kinds[1:], nil
}

func quoteLines(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&b, "\t%q\n", l)
	}
	return b.String()
}
