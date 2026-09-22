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
)

// Print emits doc formatted.
func Print(doc *block.Document) []byte { return emit(doc, false) }

// PrintRaw emits doc with every rule off, which reproduces the source byte for
// byte. It walks the tree like Print, so the identity check it serves measures
// the tree rather than the source it was cut from. Format never offers it:
// turning the rules off is a test instrument, not a mode of the formatter.
func PrintRaw(doc *block.Document) []byte { return emit(doc, true) }

func emit(doc *block.Document, raw bool) []byte {
	p := printer{src: doc.Src, raw: raw}
	p.span(doc.BOM)
	p.nodes(doc.Nodes, doc.Tail)
	return p.out.Bytes()
}

type printer struct {
	src []byte
	raw bool
	out bytes.Buffer
}

// span emits the source it covers, minus the trailing whitespace of each of
// its lines.
func (p *printer) span(s block.Span) {
	if p.raw {
		p.write(s)
		return
	}
	p.out.Write(block.TrimTrailing(p.src[s.Start:s.End]))
}

// write emits the source it covers as it stands, for the nodes no line-local
// rule may touch either.
func (p *printer) write(s block.Span) { p.out.Write(p.src[s.Start:s.End]) }

func (p *printer) nodes(nodes []block.Node, tail block.Gap) {
	for _, node := range nodes {
		p.node(node)
	}
	p.gap(tail)
}

// node emits one node. Its own gap is the one between its last metadata line
// and the block, so where it has metadata the gap in front of it is the first
// metadata line's.
func (p *printer) node(node block.Node) {
	for _, meta := range node.Meta() {
		p.gap(meta.Gap)
		p.span(meta.Lines)
	}
	p.gap(node.Gap())
	p.body(node)
}

func (p *printer) gap(g block.Gap) { p.span(g.Span) }

// body dispatches to the function owning the type. The default emits the
// node's lines unchanged, which is what a node keeps doing until a rule claims
// it.
func (p *printer) body(node block.Node) {
	switch node := node.(type) {
	case *block.Container:
		p.container(node)
	case *block.List:
		p.list(node)
	case *block.ListItem:
		p.item(node)
	case *block.Header:
		p.header(node)
	case *block.Heading:
		p.title(node, node.Level, node.Title)
	case *block.Setext:
		p.title(node, node.Level, node.Title)
	case *block.FrontMatter:
		// YAML, not AsciiDoc: trailing whitespace inside a block scalar is
		// content there.
		p.write(node.Lines())
	default:
		p.span(node.Lines())
	}
}

func (p *printer) container(node *block.Container) {
	p.span(node.Delim.Open)
	p.nodes(node.Children, node.Tail)
	p.span(node.Delim.Close)
}

func (p *printer) list(node *block.List) {
	for _, item := range node.Items {
		p.node(item)
	}
}

func (p *printer) item(node *block.ListItem) {
	p.span(node.Principal)
	for _, child := range node.Children {
		p.node(child)
	}
}
