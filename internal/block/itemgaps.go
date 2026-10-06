package block

import (
	"cmp"
	"slices"
)

// printerNotes are what reading the list items found for the printer: the
// blank-line runs and the literal paragraphs that decide which lines an item
// takes, see freezeItemGaps.
type printerNotes struct {
	// runs are where runs of blank lines end that one blank line would not
	// replace, see noteRun.
	runs []int
	// literals are the literal paragraphs collected whole, see takeLiteral.
	literals []itemLiteral
}

// itemLiteral is a literal paragraph with the item that collected it.
type itemLiteral struct {
	item  *ListItem
	lines Span
}

func (n *printerNotes) addLiterals(item *ListItem, literals []Span) {
	for _, literal := range literals {
		n.literals = append(n.literals, itemLiteral{item, literal})
	}
}

// freezeItemGaps freezes the gaps whose blank lines decide which lines an item
// takes. It runs once the tree stands, because such a gap may stand below the
// item that found it: in front of the next item, or of the block under the
// list.
func (p *parser) freezeItemGaps(nodes []Node) {
	p.notes.freezeRuns(nodes)
	p.notes.freezeUnderLiterals(nodes)
	p.notes.freezeInsideLiterals()
	p.freezeAboveIndented(nodes, false)
}

// freezeRuns freezes the runs of blank lines that one blank line would not
// replace.
func (n *printerNotes) freezeRuns(nodes []Node) {
	ends := map[int]bool{}
	for _, end := range n.runs {
		ends[end] = true
	}
	eachNode(nodes, func(node Node) {
		for _, gap := range node.gaps() {
			gap.Frozen = gap.Frozen || (!gap.Span.Empty() && ends[gap.Span.End])
		}
	})
}

// freezeUnderLiterals freezes the blank lines between two items under a
// literal paragraph. They end the paragraph; the printer may drop them between
// items, and the next item would join it.
func (n *printerNotes) freezeUnderLiterals(nodes []Node) {
	ends := map[int]bool{}
	for _, literal := range n.literals {
		ends[literal.lines.End] = true
	}
	eachNode(nodes, func(node Node) {
		list, ok := node.(*List)
		if !ok {
			return
		}
		for _, item := range list.Items[1:] {
			gap := &item.gap
			gap.Frozen = gap.Frozen || (!gap.Span.Empty() && ends[gap.Span.Start])
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
			eachNode(below(child), func(node Node) {
				above := node.gapAbove()
				inside := literal.lines.Start < above.Span.End && above.Span.End < literal.lines.End
				above.Frozen = above.Frozen || inside
			})
		}
	}
}

// freezeAboveIndented freezes the blank lines between metadata and an
// indented line in a list item, unless the line opens an item of a nested
// list. Below them, the item takes the lines down to the next blank line
// whole, see takeLiteral; without them, one by one.
func (p *parser) freezeAboveIndented(nodes []Node, inItem bool) {
	for _, node := range nodes {
		blockGap, lines := node.blockGap(), node.Lines()
		if inItem && len(node.Meta()) > 0 && !blockGap.Span.Empty() && !lines.Empty() {
			first := p.lineAt(lines.Start)
			indented := isIndented(p.text(first)) && !opensAnyList(p.src, first)
			blockGap.Frozen = blockGap.Frozen || indented
		}
		belowInItem := inItem
		switch node.(type) {
		case *ListItem:
			belowInItem = true
		case *Container:
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
