package printer

import "github.com/ypfaff/adocfmt/internal/block"

// minWidth is the shortest fence Asciidoctor reads as one, for every form but
// the two this rule leaves alone.
const minWidth = 4

// fenceWidths picks the width every fence is written at.
//
// It runs before printing and bottom up, because a container may only take a
// width no fence inside it comes out at, and printing emits its opening line
// before its children are text. An entry stands for a fence the rule writes,
// and for no other, so a node left out of the map prints as it stands.
func fenceWidths(doc *block.Document) map[block.Node]int {
	f := fencer{src: doc.Src, widths: map[block.Node]int{}}
	f.nodes(doc.Nodes)
	return f.widths
}

type fencer struct {
	src    []byte
	widths map[block.Node]int
}

func (f *fencer) nodes(nodes []block.Node) {
	for _, node := range nodes {
		f.node(node)
	}
}

// node picks the width of every fence the node holds before its own, and picks
// none at all inside a frozen one, whose lines the printer emits as they stand,
// see printer.frozen.
func (f *fencer) node(node block.Node) {
	if node.Frozen() {
		return
	}
	switch node := node.(type) {
	case *block.Container:
		f.nodes(node.Children)
		f.pick(node, node.Delim)
	case *block.Verbatim:
		f.pick(node, node.Delim)
	case *block.Table:
		f.pick(node, node.Delim)
	case *block.List:
		for _, item := range node.Items {
			f.node(item)
		}
	case *block.ListItem:
		f.nodes(node.Children)
	}
}

// pick records the width the fence is written at, the smallest one no line
// inside the block comes out at, and records nothing where the rule leaves the
// fence as it is.
func (f *fencer) pick(node block.Node, delim block.Delimiter) {
	if !rewritable(node, delim) {
		return
	}
	taken := map[int]bool{}
	if container, ok := node.(*block.Container); ok {
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
func rewritable(node block.Node, delim block.Delimiter) bool {
	if delim.Width < minWidth || delim.Char == '/' || delim.Extensible || !delim.Closed() {
		return false
	}
	return !above(node).Frozen
}

// below adds the width of every line printed inside a container that would
// close it. Asciidoctor carries the closing line of a block into the blocks
// nested in it, so a line deep inside one ends the block around it as well, and
// only a fence the rule rewrites comes out at a width its source does not have.
func (f *fencer) below(delim block.Delimiter, nodes []block.Node, taken map[int]bool) {
	for _, node := range nodes {
		for _, meta := range node.Meta() {
			f.lines(delim, meta.Lines, taken)
		}
		switch node := node.(type) {
		case *block.Container:
			f.fenced(delim, node, node.Delim, taken)
			f.below(delim, node.Children, taken)
		case *block.Verbatim:
			f.fenced(delim, node, node.Delim, taken)
			f.lines(delim, body(node, node.Delim), taken)
		case *block.Table:
			f.fenced(delim, node, node.Delim, taken)
			f.lines(delim, body(node, node.Delim), taken)
		case *block.List:
			items := make([]block.Node, len(node.Items))
			for at, item := range node.Items {
				items[at] = item
			}
			f.below(delim, items, taken)
		case *block.ListItem:
			f.lines(delim, node.Principal, taken)
			f.below(delim, node.Children, taken)
		default:
			f.lines(delim, node.Lines(), taken)
		}
	}
}

// fenced adds the width the fence of a nested block comes out at, which is the
// one the rule picked for it where it picked one.
func (f *fencer) fenced(delim block.Delimiter, node block.Node, inner block.Delimiter, taken map[int]bool) {
	width, ok := f.widths[node]
	if !ok {
		width = inner.Width
	}
	if closing, ok := delim.Fences(inner.Line(width)); ok {
		taken[closing] = true
	}
}

func (f *fencer) lines(delim block.Delimiter, span block.Span, taken map[int]bool) {
	for _, line := range block.Lines(f.src[span.Start:span.End]) {
		if width, ok := delim.Fences(line); ok {
			taken[width] = true
		}
	}
}

// body is what a delimited block holds between its two fences, and the whole
// node where it never closed, which only happens in a document the formatter
// refuses anyway.
func body(node block.Node, delim block.Delimiter) block.Span {
	if !delim.Closed() {
		return node.Lines()
	}
	return block.Span{Start: delim.Open.End, End: delim.Close.Start}
}

// above is the gap in front of a node, which is the first metadata line's where
// the node carries metadata, since the metadata is part of what stands there.
func above(node block.Node) block.Gap {
	if meta := node.Meta(); len(meta) > 0 {
		return meta[0].Gap
	}
	return node.Gap()
}

// fence emits one of the two delimiter lines, at the width the rule picked.
func (p *printer) fence(node block.Node, delim block.Delimiter, line block.Span) {
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
func (p *printer) delimited(node block.Node, delim block.Delimiter) {
	if !delim.Closed() {
		p.span(node.Lines())
		return
	}
	p.fence(node, delim, delim.Open)
	p.span(block.Span{Start: delim.Open.End, End: delim.Close.Start})
	p.fence(node, delim, delim.Close)
}
