package renderequivalence

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/corpus"
)

// TestStressedCases checks the scanner's classification against Asciidoctor
// while no rule exists to do it. Printing the tree unchanged makes every
// misread block invisible to the render checks, so this test rewrites each
// block as hard as any planned rule ever will, and the checks turn red wherever
// the scanner read a block differently from Asciidoctor.
//
// Cases in stressQuarantine are known to fail. One of them passing fails the
// test too, so the list tracks the scanner instead of drifting from it.
func TestStressedCases(t *testing.T) {
	if testing.Short() {
		t.Skip("rendering every case takes about half a minute")
	}
	if _, err := exec.LookPath("asciidoctor"); err != nil {
		t.Fatal("asciidoctor is not on the PATH")
	}

	files, err := corpus.Files(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		name := corpus.Name(casesDir, path)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if reason, ok := notRenderable[name]; ok {
				t.Skip(reason)
			}

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := block.Scan(src)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := Differences(src, stress(doc))
			if err != nil {
				t.Fatal(err)
			}

			_, quarantined := stressQuarantine[name]
			switch {
			case quarantined && len(findings) == 0:
				t.Error("passes; remove it from stressQuarantine")
			case quarantined:
				t.Skip(stressQuarantine[name])
			default:
				for _, finding := range findings {
					t.Error(finding)
				}
			}
		})
	}
}

// stress prints the tree the way the harshest rules would: every paragraph on
// one line, every heading in canonical form, every gap a rule may touch at one
// blank line. Whatever a rule must not touch stays as it is.
func stress(doc *block.Document) []byte {
	s := stresser{src: doc.Src, eol: "\n"}
	if bytes.Contains(doc.Src, []byte("\r\n")) {
		s.eol = "\r\n"
	}
	s.span(doc.BOM)
	s.nodes(doc.Nodes, doc.Tail)
	return s.out.Bytes()
}

type stresser struct {
	src []byte
	eol string
	out bytes.Buffer
}

func (s *stresser) span(sp block.Span) { s.out.Write(s.src[sp.Start:sp.End]) }

func (s *stresser) text(sp block.Span) string { return string(s.src[sp.Start:sp.End]) }

func (s *stresser) nodes(nodes []block.Node, tail block.Gap) {
	for i, node := range nodes {
		aroundHeading := isHeading(node) || (i > 0 && isHeading(nodes[i-1]))
		s.node(node, aroundHeading)
	}
	s.gap(tail, false)
}

func isHeading(node block.Node) bool {
	switch node.(type) {
	case *block.Heading, *block.Setext:
		return true
	default:
		return false
	}
}

// gap emits a gap the way the blank line rules would: a run of blank lines
// collapses to one, and a heading gets one on each side. A frozen gap stays.
func (s *stresser) gap(g block.Gap, aroundHeading bool) {
	switch {
	case g.Frozen:
		s.span(g.Span)
	case aroundHeading || !g.Span.Empty():
		s.out.WriteString(s.eol)
	}
}

func (s *stresser) node(node block.Node, aroundHeading bool) {
	c := common(node)
	for _, meta := range c.meta {
		s.gap(meta.Gap, false)
		s.span(meta.Lines)
	}
	s.gap(c.gap, aroundHeading)

	switch node := node.(type) {
	case *block.Paragraph:
		if node.Frozen {
			s.span(node.Lines)
			return
		}
		s.paragraph(node.Lines)
	case *block.Heading:
		s.heading(node.Frozen, node.Level, node.Title, node.Lines)
	case *block.Setext:
		s.heading(node.Frozen, node.Level, node.Title, node.Lines)
	case *block.Container:
		s.span(node.Delim.Open)
		s.nodes(node.Children, node.Tail)
		s.span(node.Delim.Close)
	case *block.List:
		for _, item := range node.Items {
			s.node(item, false)
		}
	case *block.ListItem:
		s.span(node.Principal)
		for _, child := range node.Children {
			s.node(child, false)
		}
	default:
		s.span(c.lines)
	}
}

// paragraph joins the lines of a paragraph into one, keeping a hard line break
// where the source has one, since that is content rather than layout.
func (s *stresser) paragraph(lines block.Span) {
	var joined []string
	for _, line := range strings.Split(strings.TrimSuffix(s.text(lines), s.eol), s.eol) {
		if n := len(joined); n > 0 && !strings.HasSuffix(joined[n-1], " +") {
			joined[n-1] += " " + line
		} else {
			joined = append(joined, line)
		}
	}
	s.out.WriteString(strings.Join(joined, s.eol) + s.eol)
}

func (s *stresser) heading(frozen bool, level int, title, lines block.Span) {
	if frozen {
		s.span(lines)
		return
	}
	s.out.WriteString(strings.Repeat("=", level+1) + " " + s.text(title) + s.eol)
}

// fields is what every node carries; the block package does not export the
// embedded struct, so the test reads it through a type switch.
type fields struct {
	gap   block.Gap
	meta  []block.Meta
	lines block.Span
}

func common(node block.Node) fields {
	switch n := node.(type) {
	case *block.Header:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Heading:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Setext:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Paragraph:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Literal:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Verbatim:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Container:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Table:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.List:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.ListItem:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Continuation:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Attribute:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Directive:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.FrontMatter:
		return fields{n.Gap, n.Meta, n.Lines}
	case *block.Opaque:
		return fields{n.Gap, n.Meta, n.Lines}
	default:
		panic("unknown node type")
	}
}

// stressQuarantine names the cases the scanner still reads differently from
// Asciidoctor, by cause. Fixing the scanner shrinks it; see TestStressedCases.
var stressQuarantine = byCase(map[string][]string{
	"a lone + ends a paragraph in Asciidoctor, the scanner reads it as text": {
		"lists/0014-should-continue-to-parse-blocks-attached-by-a-list-continuat",
		"lists/0046-adjacent-list-continuation-line-attaches-following-paragraph",
		"lists/0059-consecutive-list-continuation-lines-are-folded",
		"lists/0095-paragraph-attached-by-a-list-continuation-on-either-side-in-",
		"lists/0096-paragraph-attached-by-a-list-continuation-on-either-side-to-",
		"paragraphs/0009-normal-paragraph-terminates-at-list-continuation",
		"paragraphs/0023-quote-paragraph-terminates-at-list-continuation",
	},
	"a comment line inside a paragraph is read as text": {
		"blocks/0002-adjacent-line-comment-between-paragraphs",
		"manpage/0004-should-normalize-whitespace-and-skip-line-comments-before-an",
		"sections/0054-should-add-level-offset-to-section-level",
	},
	"the comment check counts ///, which Asciidoctor reads as text": {
		"parser/0018-break-header-at-line-with-three-forward-slashes",
	},
	"a quoted paragraph with an attribution line is a quote block": {
		"blocks/0032-quoted-paragraph-style-quote-block-with-attribution",
		"blocks/0033-should-parse-credit-line-in-quoted-paragraph-style-quote-blo",
	},
	"hardbreaks make every line break content": {
		"paragraphs/0015-should-add-a-hardbreak-at-end-of-each-line-when-hardbreaks-o",
		"paragraphs/0016-should-be-able-to-toggle-hardbreaks-by-setting-hardbreaks-op",
	},
	"roman numeral list markers are not recognized": {
		"lists/0064-should-allow-list-style-to-be-specified-explicitly-when-usin",
		"lists/0075-should-warn-if-explicit-uppercase-roman-numerals-in-list-are",
		"lists/0076-should-warn-if-explicit-lowercase-roman-numerals-in-list-are",
	},
	"an escaped directive is unescaped only at the start of a line": {
		"reader/0046-escaped-include-directive-is-left-unprocessed",
		"reader/0081-escaped-ifdef-is-unescaped-and-ignored",
	},
	"attribute-missing drop-line and {set:} make the line the unit of meaning": {
		"attributes/0017-ignores-lines-with-bad-attributes-if-attribute-missing-is-dr",
		"attributes/0018-should-drop-line-with-reference-to-missing-attribute-if-attr",
		"attributes/0020-should-drop-line-with-attribute-unassignment-by-default",
	},
	"a fenced code block with a language is not recognized": {
		"blocks/0176-should-support-fenced-code-blocks-with-languages",
		"blocks/0177-should-support-fenced-code-blocks-with-languages-and-numberi",
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
