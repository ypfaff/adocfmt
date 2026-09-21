// Package printer emits the block tree as AsciiDoc.
//
// Printing is one recursive traversal, with one function per node type. Every
// formatting opinion becomes a branch in the function for the node it applies
// to, or, where the opinion is line-local, in span, the one place every
// emitted byte passes through.
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
	p.span(tail.Span)
}

// node dispatches to the function owning the type. The default emits the node
// unchanged, which is what a node keeps doing until a rule claims it.
func (p *printer) node(node block.Node) {
	switch node := node.(type) {
	case *block.Container:
		p.container(node)
	case *block.List:
		p.list(node)
	case *block.FrontMatter:
		// YAML, not AsciiDoc: trailing whitespace inside a block scalar is
		// content there.
		p.write(node.Extent())
	default:
		p.span(node.Extent())
	}
}

func (p *printer) container(node *block.Container) {
	p.span(block.Span{Start: node.Extent().Start, End: node.Delim.Open.End})
	p.nodes(node.Children, node.Tail)
	p.span(node.Delim.Close)
}

func (p *printer) list(node *block.List) {
	p.span(block.Span{Start: node.Extent().Start, End: node.Lines().Start})
	for _, item := range node.Items {
		p.item(item)
	}
}

func (p *printer) item(node *block.ListItem) {
	p.span(block.Span{Start: node.Extent().Start, End: node.Principal.End})
	for _, child := range node.Children {
		p.node(child)
	}
}
