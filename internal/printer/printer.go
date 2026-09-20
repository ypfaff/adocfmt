// Package printer emits the block tree as AsciiDoc.
//
// Printing is one recursive traversal, with one function per node type. Every
// formatting opinion becomes an option-guarded branch in the function for the
// node it applies to.
package printer

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/block"
)

// Print emits doc. With no rule implemented, every node takes the raw branch,
// which is why the output equals the input byte for byte.
func Print(doc *block.Document) []byte {
	p := printer{src: doc.Src}
	p.span(doc.BOM)
	p.nodes(doc.Nodes, doc.Tail)
	return p.out.Bytes()
}

type printer struct {
	src []byte
	out bytes.Buffer
}

func (p *printer) span(s block.Span) { p.out.Write(p.src[s.Start:s.End]) }

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
	p.span(block.Span{Start: node.Extent().Start, End: node.Lines.Start})
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
