package printer

import (
	"slices"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/tree"
)

// minWidth is the shortest fence Asciidoctor reads as one, for every form but
// the two this rule leaves alone.
const minWidth = 4

// fenceWidths picks the width every fence is written at.
//
// It runs before printing and bottom up, because a container may only take a
// width no fence inside it comes out at, and printing emits its opening line
// before its children are text. An entry stands for a fence the rule writes,
// and for no other, so a node left out of the map prints as it stands.
func fenceWidths(doc *tree.Document) map[tree.Node]int {
	f := fencer{src: doc.Src, widths: map[tree.Node]int{}}
	f.nodes(doc.Nodes, nil)
	return f.widths
}

type fencer struct {
	src    []byte
	widths map[tree.Node]int
}

func (f *fencer) nodes(nodes []tree.Node, around []tree.Delimiter) {
	for _, node := range nodes {
		f.node(node, around)
	}
}

// node picks the width of every fence the node holds before its own, and picks
// none at all inside a frozen one, whose lines the printer emits as they stand,
// see printer.frozen.
func (f *fencer) node(node tree.Node, around []tree.Delimiter) {
	if node.Base().Frozen {
		return
	}
	switch node := node.(type) {
	case *tree.Container:
		f.nodes(node.Children, enclose(around, node, node.Delim))
		f.pick(node, node.Delim, around)
	case *tree.Verbatim:
		f.pick(node, node.Delim, around)
	case *tree.Table:
		f.pick(node, node.Delim, around)
	case *tree.List:
		for _, item := range node.Items {
			f.node(item, around)
		}
	case *tree.ListItem:
		f.nodes(node.Children, around)
	}
}

// enclose carries a fence down to the blocks inside it, so that none of them
// comes out on its line. A fence the rule writes needs no carrying: it picks
// after them and gives way to whatever they came out at.
func enclose(around []tree.Delimiter, node tree.Node, delim tree.Delimiter) []tree.Delimiter {
	if rewritable(node, delim) {
		return around
	}
	return append(slices.Clip(around), delim)
}

// pick records the width the fence is written at, the smallest one neither a
// line inside the block comes out at nor a fence around it stands at, and
// records nothing where the rule leaves the fence as it is.
func (f *fencer) pick(node tree.Node, delim tree.Delimiter, around []tree.Delimiter) {
	if !rewritable(node, delim) {
		return
	}
	taken := map[int]bool{}
	for _, outer := range around {
		if _, ok := block.Fences(outer, delim.Line(outer.Width)); ok {
			taken[outer.Width] = true
		}
	}
	if container, ok := node.(*tree.Container); ok {
		f.below(delim, container.Children, taken)
	} else {
		f.lines(delim, body(node, delim), taken)
	}

	width := minWidth
	for taken[width] {
		width++
	}
	f.widths[node] = width
}

// rewritable reports whether the rule may write this fence. An open block and a
// Markdown fence have one legal width each. A comment block is a node of its
// own only where it stands alone, so rewriting one would turn on where it sits.
// A directive, inside the block or right above it, may bring in the very line a
// shorter fence would close on. A frozen node never gets this far, see node.
func rewritable(node tree.Node, delim tree.Delimiter) bool {
	if delim.Width < minWidth || delim.Char == '/' || delim.Extensible || !delim.Closed() {
		return false
	}
	return !above(node).BesideDirective
}

// below adds the width of every line printed inside a container that would
// close it. Asciidoctor carries the closing line of a block into the blocks
// nested in it, so a line deep inside one ends the block around it as well, and
// only a fence the rule rewrites comes out at a width its source does not have.
func (f *fencer) below(delim tree.Delimiter, nodes []tree.Node, taken map[int]bool) {
	for _, node := range nodes {
		for _, meta := range node.Base().Meta {
			f.lines(delim, meta.Lines, taken)
		}
		switch node := node.(type) {
		case *tree.Container:
			f.fenced(delim, node, node.Delim, taken)
			f.below(delim, node.Children, taken)
		case *tree.Verbatim:
			f.fenced(delim, node, node.Delim, taken)
			f.lines(delim, body(node, node.Delim), taken)
		case *tree.Table:
			f.fenced(delim, node, node.Delim, taken)
			f.lines(delim, body(node, node.Delim), taken)
		case *tree.List:
			items := make([]tree.Node, len(node.Items))
			for at, item := range node.Items {
				items[at] = item
			}
			f.below(delim, items, taken)
		case *tree.ListItem:
			f.lines(delim, node.Principal, taken)
			f.below(delim, node.Children, taken)
		default:
			f.lines(delim, node.Base().Lines, taken)
		}
	}
}

// fenced adds the width the fence of a nested block comes out at, which is the
// one the rule picked for it where it picked one.
func (f *fencer) fenced(delim tree.Delimiter, node tree.Node, inner tree.Delimiter, taken map[int]bool) {
	width, ok := f.widths[node]
	if !ok {
		width = inner.Width
	}
	if closing, ok := block.Fences(delim, inner.Line(width)); ok {
		taken[closing] = true
	}
}

func (f *fencer) lines(delim tree.Delimiter, span tree.Span, taken map[int]bool) {
	for _, line := range block.Lines(f.src[span.Start:span.End]) {
		if width, ok := block.Fences(delim, line); ok {
			taken[width] = true
		}
	}
}

// body is what a delimited block holds between its two fences, and the whole
// node where it never closed, which only happens in a document the formatter
// refuses anyway.
func body(node tree.Node, delim tree.Delimiter) tree.Span {
	if !delim.Closed() {
		return node.Base().Lines
	}
	return tree.Span{Start: delim.Open.End, End: delim.Close.Start}
}

// above is the gap in front of a node, which is the first metadata line's where
// the node carries metadata, since the metadata is part of what stands there.
func above(node tree.Node) tree.Gap {
	if meta := node.Base().Meta; len(meta) > 0 {
		return meta[0].Gap
	}
	return node.Base().Gap
}

// fence emits one of the two delimiter lines, at the width the rule picked.
func (p *printer) fence(node tree.Node, delim tree.Delimiter, line tree.Span) {
	width, ok := p.widths[node]
	if !ok || line.Empty() {
		p.span(line)
		return
	}
	p.out.Write(delim.Line(width))
	p.out.Write(block.Terminator(p.src[line.Start:line.End]))
}

// delimited emits a block holding no nodes: its two fences, and between them
// the source, which only the line-local rules reach.
func (p *printer) delimited(node tree.Node, delim tree.Delimiter) {
	if !delim.Closed() {
		p.span(node.Base().Lines)
		return
	}
	p.fence(node, delim, delim.Open)
	p.span(tree.Span{Start: delim.Open.End, End: delim.Close.Start})
	p.fence(node, delim, delim.Close)
}
