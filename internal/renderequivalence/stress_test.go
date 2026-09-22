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
// block the way the planned rules will, and the checks turn red wherever the
// scanner read a block differently from Asciidoctor.
func TestStressedCases(t *testing.T) {
	if testing.Short() {
		t.Skip("renders every case with Asciidoctor")
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
			// The tree of a document the formatter refuses is not one to stress.
			if reason, ok := refused[name]; ok {
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
			for _, finding := range findings {
				t.Error(finding)
			}
		})
	}
}

// stress prints the tree the way the harshest rules would: every paragraph on
// one line and every heading in canonical form. Whatever a rule must not touch
// stays as it is.
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
	for _, node := range nodes {
		s.node(node)
	}
	s.span(tail.Span)
}

func (s *stresser) node(node block.Node) {
	for _, meta := range node.Meta() {
		s.span(meta.Gap.Span)
		s.span(meta.Lines)
	}
	s.span(node.Gap().Span)

	switch node := node.(type) {
	case *block.Paragraph:
		if node.Frozen() {
			s.span(node.Lines())
			return
		}
		s.paragraph(node.Lines())
	case *block.Heading:
		s.heading(node.Frozen(), node.Level, node.Title, node.Lines())
	case *block.Setext:
		s.heading(node.Frozen(), node.Level, node.Title, node.Lines())
	case *block.Container:
		s.span(node.Delim.Open)
		s.nodes(node.Children, node.Tail)
		s.span(node.Delim.Close)
	case *block.List:
		for _, item := range node.Items {
			s.node(item)
		}
	case *block.ListItem:
		s.span(node.Principal)
		for _, child := range node.Children {
			s.node(child)
		}
	default:
		s.span(node.Lines())
	}
}

// paragraph joins the lines of a paragraph into one, keeping a hard line break
// where the source has one, since that is content rather than layout.
// Asciidoctor strips trailing whitespace before it looks for the break.
func (s *stresser) paragraph(lines block.Span) {
	var joined []string
	for _, line := range strings.Split(strings.TrimSuffix(s.text(lines), s.eol), s.eol) {
		if n := len(joined); n > 0 && !strings.HasSuffix(strings.TrimRight(joined[n-1], " \t"), " +") {
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
