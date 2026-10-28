// Package printer emits the block tree as AsciiDoc.
//
// Printing is one recursive traversal, with one function per node type. Every
// formatting opinion becomes a branch in the function for the node it applies
// to, or, where the opinion is line-local, in span, the one place every byte of
// source passes through. A gap is decided in the traversal, because what it
// holds depends on the nodes on both sides of it.
package printer

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/tree"
)

// Print emits doc formatted.
func Print(doc *tree.Document) []byte { return emit(doc, false) }

// PrintRaw emits doc with every rule off, which reproduces the source byte for
// byte. It walks the tree like Print, so the identity check it serves measures
// the tree rather than the source it was cut from. Format never offers it:
// turning the rules off is a test instrument, not a mode of the formatter.
func PrintRaw(doc *tree.Document) []byte { return emit(doc, true) }

func emit(doc *tree.Document, raw bool) []byte {
	p := printer{src: doc.Src, raw: raw, eol: string(doc.LineEnding)}
	if !raw {
		p.widths = fenceWidths(doc)
		p.markers = listMarkers(doc)
	}
	p.span(doc.BOM)
	p.start = p.out.Len()
	p.nodes(doc.Nodes, doc.Tail, betweenSiblings)
	p.finish()
	return p.out.Bytes()
}

type printer struct {
	src []byte
	raw bool
	eol string
	// start is where the first line begins, past the byte order mark, so a
	// document holding no line at all is told from one whose last line is left
	// open.
	start int
	// widths holds the width every fence is written at, see fenceWidths.
	widths map[tree.Node]int
	// markers holds the marker every list item is written with, see
	// listMarkers.
	markers map[tree.Node]string
	out     bytes.Buffer
}

// span emits the source it covers, minus the trailing whitespace of each of
// its lines.
func (p *printer) span(s tree.Span) {
	if p.raw {
		p.write(s)
		return
	}
	p.out.Write(block.TrimTrailing(p.src[s.Start:s.End]))
}

// write emits the source it covers as it stands, for the nodes no line-local
// rule may touch either.
func (p *printer) write(s tree.Span) { p.out.Write(p.src[s.Start:s.End]) }

func (p *printer) nodes(nodes []tree.Node, tail tree.Gap, w where) {
	var prev tree.Node
	for _, node := range nodes {
		p.node(node, prev, w)
		prev = node
	}
	p.tail(tail)
}

// node emits one node. Its own gap is the one between its last metadata line
// and the block, so where it has metadata the gap in front of it is the first
// metadata line's.
func (p *printer) node(node, prev tree.Node, w where) {
	meta := node.Base().Meta
	if len(meta) == 0 {
		p.gap(node.Base().Gap, prev, w)
		p.body(node)
		return
	}

	p.gap(meta[0].Gap, prev, w)
	p.span(meta[0].Lines)
	above := meta[0]
	for _, m := range meta[1:] {
		p.bound(m.Gap, above)
		p.span(m.Lines)
		above = m
	}
	p.bound(node.Base().Gap, above)
	p.body(node)
}

// body dispatches to the function owning the type. The default emits the
// node's lines unchanged, which is what a node keeps doing until a rule claims
// it.
func (p *printer) body(node tree.Node) {
	switch node := node.(type) {
	case *tree.Container:
		p.container(node)
	case *tree.Verbatim:
		p.delimited(node, node.Delim)
	case *tree.Table:
		p.delimited(node, node.Delim)
	case *tree.Paragraph, *tree.Admonition:
		p.prose(node)
	case *tree.List:
		p.list(node)
	case *tree.ListItem:
		p.item(node)
	case *tree.Header:
		p.header(node)
	case *tree.Heading:
		p.title(node, node.Level, node.Title)
	case *tree.Setext:
		p.title(node, node.Level, node.Title)
	case *tree.FrontMatter:
		// YAML, not AsciiDoc: trailing whitespace inside a block scalar is
		// content there.
		p.write(node.Lines)
	default:
		p.span(node.Base().Lines)
	}
}

// frozen emits a node whose lines have to stay as they are, and reports that it
// did. The nodes holding other nodes ask, because a rule reaches the gaps
// between their children through the traversal. Raw printing descends instead,
// so the identity check keeps measuring every child.
func (p *printer) frozen(node tree.Node) bool {
	if p.raw || !node.Base().Frozen {
		return false
	}
	p.span(node.Base().Lines)
	return true
}

// container starts over: its delimiters confine what is inside, so a list item
// holding it does not reach past them.
func (p *printer) container(node *tree.Container) {
	if p.frozen(node) {
		return
	}
	p.fence(node, node.Delim, node.Delim.Open)
	p.nodes(node.Children, node.Tail, betweenSiblings)
	p.fence(node, node.Delim, node.Delim.Close)
}
