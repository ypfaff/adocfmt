package printer

import (
	"slices"

	"github.com/ypfaff/adocfmt/internal/block"
)

// canonical is the marker a list is written with, where the rule writes one at
// all. Every other marker stays as it is: a., A., i), I), the bullet, a
// description term and a callout say something the rewrite would drop.
func canonical(marker string) (string, bool) {
	switch marker {
	case "-":
		return "*", true
	case "1.":
		return ".", true
	}
	return "", false
}

// listMarkers picks the marker every list item is written with.
//
// It runs before printing, because the marker is the identity of the whole
// list while the printer emits one item at a time, and because the decision
// rests on the lists around the one being written. An entry stands for an item
// the rule writes, and for no other, so an item left out of the map prints as
// it stands.
func listMarkers(doc *block.Document) map[block.Node]string {
	k := picker{src: doc.Src, markers: map[block.Node]string{}}
	k.nodes(doc.Nodes)
	return k.markers
}

type picker struct {
	src     []byte
	markers map[block.Node]string
}

// nodes walks a run of siblings. What a list among them stands in, it carries
// itself, so the walk only has to hand it the node underneath, which a
// directive there reaches it from.
func (k *picker) nodes(nodes []block.Node) {
	for at, node := range nodes {
		switch node := node.(type) {
		case *block.Container:
			k.nodes(node.Children)
		case *block.List:
			var next block.Node
			if at+1 < len(nodes) {
				next = nodes[at+1]
			}
			k.list(node, next)
		}
	}
}

func (k *picker) list(list *block.List, next block.Node) {
	if list.Frozen() {
		return
	}
	for _, item := range list.Items {
		k.nodes(item.Children)
	}
	if k.reached(list, next) {
		return
	}

	var picked string
	if canon, ok := canonical(list.Marker); ok && k.rewritable(list, canon) {
		picked = canon
	}
	// Only the first line of a list can become a section title, and only where
	// the list stands at section level. Where writing it would make one, that
	// item prints as it stands, and the rest of the list keeps the marker the
	// item carries: a second marker there would nest the rest under it.
	items := list.Items
	if list.Section && k.underlines(list, items[0], picked) {
		picked, items = "", items[1:]
	}
	for _, item := range items {
		k.markers[item] = written(k.src, item, picked)
	}
}

// written is the marker an item is written with: the one the rule picked, or
// the one the item stands on. That is not List.Marker, which holds the key the
// items are compared on, "1." for an item that says "7.".
func written(src []byte, item *block.ListItem, picked string) string {
	if picked != "" {
		return picked
	}
	return string(src[item.Marker.Start:item.Marker.End])
}

// reached reports whether an include or a conditional reaches the list, which
// leaves every line of it as it stands, the space behind a marker included:
// what those lines mean is known only once Asciidoctor has resolved the
// directive. The scanner reports the directives it read within the list as
// List.Extensible, and a directive beside the list freezes the gap between
// them.
func (k *picker) reached(list *block.List, next block.Node) bool {
	return list.Extensible || above(list).Frozen || (next != nil && above(next).Frozen)
}

// rewritable reports whether the list may be written with the marker.
//
// Asciidoctor nests on a change of marker and ends every list down to the one
// a marker already open belongs to, so a marker in use around this list would
// move its items. A frozen item keeps the marker it has, which would leave it
// nested under the item above once the rest of the list carries another one.
func (k *picker) rewritable(list *block.List, marker string) bool {
	for _, item := range list.Items {
		if item.Frozen() {
			return false
		}
	}
	return !slices.Contains(list.Open, marker) && !k.holds(list, marker)
}

// holds reports whether a line inside the list would be an item of it once the
// list carried the marker. A line the scanner read as prose counts too:
// Asciidoctor ends a paragraph on an item of a list open around it, so the
// rewrite would turn that line into one.
func (k *picker) holds(list *block.List, marker string) bool {
	for _, item := range list.Items {
		if k.carries(item.Principal, marker) || k.heldBy(item.Children, marker) {
			return true
		}
	}
	return false
}

func (k *picker) heldBy(nodes []block.Node, marker string) bool {
	for _, node := range nodes {
		switch node := node.(type) {
		case *block.Container, *block.Verbatim, *block.Table:
			// A delimiter confines what it holds: no list open outside it is
			// open within, so no line in there becomes an item of this one.
		case *block.List:
			if k.holds(node, marker) {
				return true
			}
		default:
			if k.carries(node.Lines(), marker) {
				return true
			}
		}
	}
	return false
}

// carries reports whether a line of the span opens a list item with the marker.
func (k *picker) carries(span block.Span, marker string) bool {
	for _, line := range block.Lines(k.src[span.Start:span.End]) {
		if on, ok := block.ListMarker(line); ok && on == marker {
			return true
		}
	}
	return false
}

// underlines reports whether the item's first line, written with the marker,
// would turn the line below it into the underline of a section title.
//
// The line below is read from the source rather than from the output, because
// the two are the same line here: the only line another rule may drop between
// two items is a blank one, and what follows it is a marker, which no underline
// is. It has to stand in the list as well, since under the list the printer
// writes a blank line, and nothing underlines a blank line.
func (k *picker) underlines(list *block.List, item *block.ListItem, picked string) bool {
	if len(block.Lines(k.src[list.Lines().Start:list.Lines().End])) < 2 {
		return false
	}
	head, at := itemHead(k.src, item, written(k.src, item, picked))
	if lines := block.Lines(k.src[at:item.Principal.End]); len(lines) > 0 {
		head = append(head, lines[0]...)
	}
	return block.UnderlinesTitle(head, block.LineBelow(k.src, item.Principal.Start))
}

// itemHead builds the head of an item's first line: the indentation the marker
// stands at, the marker, and the single space separating it from the text.
// textAt is where that text begins, the whitespace behind the marker skipped.
//
// The space is left out where the item carries no text of its own, which only
// a description term does, since it would be trailing whitespace there.
func itemHead(src []byte, item *block.ListItem, marker string) (head []byte, textAt int) {
	at := item.Marker.End
	for at < len(src) && (src[at] == ' ' || src[at] == '\t') {
		at++
	}
	head = append(append([]byte{}, src[item.Principal.Start:item.Marker.Start]...), marker...)
	if at < item.Principal.End && src[at] != '\n' && src[at] != '\r' {
		head = append(head, ' ')
	}
	return head, at
}

func (p *printer) list(node *block.List) {
	if p.frozen(node) {
		return
	}
	var prev block.Node
	for _, item := range node.Items {
		p.node(item, prev, betweenItems)
		prev = item
	}
}

func (p *printer) item(node *block.ListItem) {
	if p.frozen(node) {
		return
	}
	p.principal(node)
	var prev block.Node = node
	for _, child := range node.Children {
		p.node(child, prev, insideItem)
		prev = child
	}
}

// principal emits the text the item carries itself: the marker the rule
// picked, one space, and what stands behind it. An item the rule left out
// prints as it stands.
func (p *printer) principal(node *block.ListItem) {
	marker, ok := p.markers[node]
	if !ok {
		p.span(node.Principal)
		return
	}
	head, text := itemHead(p.src, node, marker)
	p.out.Write(head)
	p.span(block.Span{Start: text, End: node.Principal.End})
}
