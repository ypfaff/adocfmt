// Package renderequivalence checks that formatting a document does not change
// what the reader of the rendered document sees.
//
// See docs/testing-strategy.adoc for the three checks and what the
// case set does and does not cover.
package renderequivalence

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
)

var (
	whitespaceRun = regexp.MustCompile(`\s+`)
	// Asciidoctor renders code and literal blocks as <pre> and never nests
	// them, so the non-greedy body is unambiguous.
	preBlock = regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)
)

// normalizeHTML collapses every run of whitespace to a single space.
//
// Where a paragraph wraps is invisible in the rendered page, because HTML shows
// any run of whitespace as one space. Comparing the renderings byte for byte
// would therefore report line breaks no reader can see. The whitespace that
// does reach the reader sits in verbatim blocks, which verbatimBlocks compares
// exactly.
func normalizeHTML(rendered string) string {
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(rendered, " "))
}

// verbatimBlocks returns the content of every code and literal block, in
// document order, with HTML entities decoded. Whitespace counts there, which is
// what lets normalizeHTML ignore it everywhere else.
func verbatimBlocks(rendered string) []string {
	matches := preBlock.FindAllStringSubmatch(rendered, -1)
	blocks := make([]string, 0, len(matches))
	for _, match := range matches {
		blocks = append(blocks, html.UnescapeString(match[1]))
	}
	return blocks
}

// comments returns every comment line of an AsciiDoc source, in order, trimmed.
// Asciidoctor drops comments before they reach the HTML, so this is the one
// check that reads the sources.
//
// Any line starting with // counts, including a line of code inside a listing
// block. Both sides are scanned the same way, so such a line reports a
// difference only when formatting really changed it.
func comments(src []byte) []string {
	var found []string
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			found = append(found, trimmed)
		}
	}
	return found
}

// Differences reports one finding per check that failed. An empty result means
// formatting preserved everything a reader sees.
func Differences(src, formatted []byte) ([]string, error) {
	// Rendering is deterministic, so identical sources cannot differ in
	// anything the checks look at, and each render is a process start.
	if bytes.Equal(src, formatted) {
		return nil, nil
	}

	before, err := render(src)
	if err != nil {
		return nil, fmt.Errorf("rendering the input: %w", err)
	}
	after, err := render(formatted)
	if err != nil {
		return nil, fmt.Errorf("rendering the output: %w", err)
	}

	var findings []string
	if got, want := normalizeHTML(after), normalizeHTML(before); got != want {
		findings = append(findings, "rendering differs:\n"+firstDifference(want, got))
	}
	if got, want := comments(formatted), comments(src); !slices.Equal(got, want) {
		findings = append(findings, fmt.Sprintf("comments differ:\ninput:  %q\noutput: %q", want, got))
	}
	if got, want := verbatimBlocks(after), verbatimBlocks(before); !slices.Equal(got, want) {
		findings = append(findings, fmt.Sprintf("verbatim content differs:\ninput:  %q\noutput: %q", want, got))
	}
	return findings, nil
}

// firstDifference reports a window around the first differing byte, so a finding
// on a long document stays readable.
func firstDifference(want, got string) string {
	const context = 60

	at := 0
	for at < len(want) && at < len(got) && want[at] == got[at] {
		at++
	}
	from := max(at-context, 0)

	window := func(s string) string { return s[from:min(at+context, len(s))] }
	return "want: ..." + window(want) + "...\ngot:  ..." + window(got) + "..."
}
