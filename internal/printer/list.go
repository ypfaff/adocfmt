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
	p := picker{src: doc.Src, picked: map[block.Node]string{}}
	p.nodes(doc.Nodes)
	return p.picked
}

type picker struct {
	src    []byte
	picked map[block.Node]string
}

// nodes walks a run of siblings. What a list among them stands in, it carries
// itself, so the walk only has to hand it the node underneath, which a
// directive there reaches it from.
func (p *picker) nodes(nodes []block.Node) {
	for at, node := range nodes {
		switch node := node.(type) {
		case *block.Container:
			p.nodes(node.Children)
		case *block.List:
			var next block.Node
			if at+1 < len(nodes) {
				next = nodes[at+1]
			}
			p.list(node, next)
		}
	}
}

func (p *picker) list(list *block.List, next block.Node) {
	if list.Frozen() {
		return
	}
	for _, item := range list.Items {
		p.nodes(item.Children)
	}

	var picked string
	if canon, ok := canonical(list.Marker); ok && p.rewritable(list, canon, next) {
		picked = canon
	}
	// Only the first line of a list can become a section title, and only where
	// the list stands at section level. Where the marker the rule picked would
	// make one, the list keeps the marker it has; where the line would be one
	// even then, that item stays as it stands and the rest of the list follows
	// the marker.
	items := list.Items
	if list.Section && p.underlines(items[0], picked) {
		picked = ""
		if p.underlines(items[0], picked) {
			items = items[1:]
		}
	}
	for _, item := range items {
		p.picked[item] = written(p.src, item, picked)
	}
}

// written is the marker an item is written with: the one the rule picked, or
// the one the item stands on, since List.Marker is the key Asciidoctor
// normalizes to rather than what the item says.
func written(src []byte, item *block.ListItem, picked string) string {
	if picked != "" {
		return picked
	}
	return string(src[item.Marker.Start:item.Marker.End])
}

// rewritable reports whether the list may be written with the marker.
//
// Asciidoctor nests on a change of marker and ends every list down to the one
// a marker already open belongs to, so a marker in use around this list would
// move its items. A frozen item keeps the marker it has, which would leave it
// nested under the item above once the rest of the list carries another one. A
// directive reaches a list from either side, and a list has no closing
// delimiter to keep what the directive brings in out of it. The scanner reports
// the directives it saw within reach as List.Extensible; the gaps around the
// list are what is left, since a directive freezes the gap it stands next to.
func (p *picker) rewritable(list *block.List, marker string, next block.Node) bool {
	if list.Extensible || above(list).Frozen || (next != nil && above(next).Frozen) {
		return false
	}
	for _, item := range list.Items {
		if item.Frozen() {
			return false
		}
	}
	return !slices.Contains(list.Open, marker) && !holds(list, marker)
}

// holds reports whether a list below this one carries the marker, which is the
// list it would close back to once this one carries the same marker. A
// delimited block confines the lists inside it, and ends the search.
func holds(list *block.List, marker string) bool {
	for _, item := range list.Items {
		for _, child := range item.Children {
			nested, ok := child.(*block.List)
			if ok && (nested.Marker == marker || holds(nested, marker)) {
				return true
			}
		}
	}
	return false
}

// underlines reports whether the item's first line, written with the marker,
// would turn the line below it into the underline of a section title.
//
// The line below is read from the source, because it is the line that ends up
// there: the blank line a rule may drop between two items is followed by a
// marker, which no underline is, and everywhere else a blank line stays.
func (p *picker) underlines(item *block.ListItem, picked string) bool {
	head, at := itemHead(p.src, item, written(p.src, item, picked))
	if lines := block.Lines(p.src[at:item.Principal.End]); len(lines) > 0 {
		head = append(head, lines[0]...)
	}
	return block.UnderlinesTitle(head, block.LineBelow(p.src, item.Principal.Start))
}

// itemHead builds the head of an item's first line: the indentation the marker
// stands at, the marker, and the single space separating it from the text.
// text is where that text begins, the whitespace behind the marker skipped.
//
// The space is left out where the item carries no text of its own, which only
// a description term does, since it would be trailing whitespace there.
func itemHead(src []byte, item *block.ListItem, marker string) (head []byte, text int) {
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
