package block

import (
	"cmp"
	"slices"

	"github.com/ypfaff/adocfmt/internal/tree"
)

// printerNotes are what reading the list items found for the printer: the
// blank lines and the literal paragraphs that decide which lines an item
// takes, see freezeItemGaps.
type printerNotes struct {
	// keptGaps are where the blank lines end that the printer has to leave as
	// they are, see noteRun and noteBareTerm.
	keptGaps []int
	// literals are the literal paragraphs collected whole, see takeLiteral.
	literals []itemLiteral
}

// itemLiteral is a literal paragraph with the item that collected it.
type itemLiteral struct {
	item  *tree.ListItem
	lines tree.Span
}

func (n *printerNotes) addLiterals(item *tree.ListItem, literals []tree.Span) {
	for _, literal := range literals {
		n.literals = append(n.literals, itemLiteral{item, literal})
	}
}

// freezeItemGaps freezes the gaps whose blank lines decide which lines an item
// takes. It runs once the tree stands, because such a gap may stand below the
// item that found it: in front of the next item, or of the block under the
// list.
func (p *parser) freezeItemGaps(nodes []tree.Node) {
	p.notes.freezeKeptGaps(nodes)
	p.notes.freezeUnderLiterals(nodes)
	p.notes.freezeInsideLiterals()
	p.freezeAboveIndented(nodes, false)
}

// freezeKeptGaps freezes the gaps noted for the printer.
func (n *printerNotes) freezeKeptGaps(nodes []tree.Node) {
	ends := map[int]bool{}
	for _, end := range n.keptGaps {
		ends[end] = true
	}
	eachNode(nodes, func(node tree.Node) {
		for _, gap := range gaps(node) {
			if !gap.Span.Empty() && ends[gap.Span.End] {
				gap.Stage = tree.Exact
			}
		}
	})
}

// freezeUnderLiterals freezes the blank lines between two items under a
// literal paragraph. They end the paragraph; the printer may drop them between
// items, and the next item would join it.
func (n *printerNotes) freezeUnderLiterals(nodes []tree.Node) {
	ends := map[int]bool{}
	for _, literal := range n.literals {
		ends[literal.lines.End] = true
	}
	eachNode(nodes, func(node tree.Node) {
		list, ok := node.(*tree.List)
		if !ok {
			return
		}
		for _, item := range list.Items[1:] {
			gap := &item.Gap
			if !gap.Span.Empty() && ends[gap.Span.Start] {
				gap.Stage = tree.Exact
			}
		}
	})
}

// freezeInsideLiterals keeps blank lines out of a literal paragraph. Where its
// lines hold further blocks, a blank line between two of them would end the
// item there. The printer adds none between the blocks of an item, but it may
// between the blocks nested in them.
func (n *printerNotes) freezeInsideLiterals() {
	for _, literal := range n.literals {
		for _, child := range literal.item.Children {
			eachNode(below(child), func(node tree.Node) {
				above := gapAbove(node)
				if literal.lines.Start < above.Span.End && above.Span.End < literal.lines.End {
					above.Stage = tree.Exact
				}
			})
		}
	}
}

// freezeAboveIndented freezes the blank lines between metadata and an
// indented line in a list item, unless the line opens an item of a nested
// list. Below them, the item takes the lines down to the next blank line
// whole, see takeLiteral; without them, one by one.
func (p *parser) freezeAboveIndented(nodes []tree.Node, inItem bool) {
	for _, node := range nodes {
		gap, lines := blockGap(node), node.Base().Lines
		if inItem && len(node.Base().Meta) > 0 && !gap.Span.Empty() && !lines.Empty() {
			first := p.lineAt(lines.Start)
			if isIndented(p.text(first)) && !opensAnyList(p.src, first) {
				gap.Stage = tree.Exact
			}
		}
		belowInItem := inItem
		switch node.(type) {
		case *tree.ListItem:
			belowInItem = true
		case *tree.Container:
			// The block confines the reader, see delimited.
			belowInItem = false
		}
		p.freezeAboveIndented(below(node), belowInItem)
	}
}

// lineAt returns the line that starts at offset. A node's lines start where a
// line does, so the search always finds one.
func (p *parser) lineAt(offset int) line {
	at, _ := slices.BinarySearchFunc(p.lines, offset, func(l line, target int) int {
		return cmp.Compare(l.full.Start, target)
	})
	return p.lines[at]
}

// gaps returns the gaps a node holds, top down: one in front of each
// metadata line, then the one between the metadata and the block.
func gaps(node tree.Node) []*tree.Gap {
	b := node.Base()
	gaps := make([]*tree.Gap, 0, len(b.Meta)+1)
	for i := range b.Meta {
		gaps = append(gaps, &b.Meta[i].Gap)
	}
	return append(gaps, &b.Gap)
}

// gapAbove is the gap in front of the whole node: in front of its first
// metadata line, or in front of the block if it has none.
func gapAbove(node tree.Node) *tree.Gap {
	b := node.Base()
	if len(b.Meta) > 0 {
		return &b.Meta[0].Gap
	}
	return &b.Gap
}

// blockGap is the gap right in front of the block, below its metadata.
func blockGap(node tree.Node) *tree.Gap { return &node.Base().Gap }

// eachNode calls visit for every node in nodes and in the nodes they hold, top
// down.
func eachNode(nodes []tree.Node, visit func(tree.Node)) {
	for _, node := range nodes {
		visit(node)
		eachNode(below(node), visit)
	}
}

// below returns the nodes the node holds: the blocks of a delimited block or a
// list item, the items of a list.
func below(node tree.Node) []tree.Node {
	switch node := node.(type) {
	case *tree.Container:
		return node.Children
	case *tree.ListItem:
		return node.Children
	case *tree.List:
		items := make([]tree.Node, len(node.Items))
		for i, item := range node.Items {
			items[i] = item
		}
		return items
	}
	return nil
}
