package block

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// dump renders the tree as one line per node, so a test states the structure it
// expects instead of reaching into fields.
func dump(doc *Document) string {
	d := dumper{doc: doc}
	d.nodes(doc.Nodes, 0)
	return d.out.String()
}

type dumper struct {
	doc *Document
	out strings.Builder
}

// writef keeps the one call that could report an error, and never does, in a
// single place.
func (d *dumper) writef(format string, args ...any) {
	_, _ = fmt.Fprintf(&d.out, format, args...)
}

func (d *dumper) nodes(nodes []Node, depth int) {
	for _, node := range nodes {
		d.node(node, depth)
	}
}

func (d *dumper) node(node Node, depth int) {
	d.writef("%s%s", strings.Repeat("  ", depth), reflect.TypeOf(node).Elem().Name())
	if node.Frozen() {
		d.writef("!")
	}
	if node.Gap().Frozen {
		d.writef(" gap!")
	}
	// A bang on the fence marks a body a directive may extend, the way it marks
	// a frozen node and a frozen gap. A list has no fence to carry it.
	if delim, ok := fenceOf(node); ok && delim.Extensible {
		d.writef(" delim!")
	}
	if list, ok := node.(*List); ok && list.Extensible {
		d.writef(" extensible!")
	}
	for _, meta := range node.Meta() {
		// A bang marks a metadata line whose gap below carries meaning, the way
		// it marks a frozen node and a frozen gap.
		if meta.Open {
			d.writef(" meta!(%s)", d.quote(meta.Lines))
			continue
		}
		d.writef(" meta(%s)", d.quote(meta.Lines))
	}

	switch node := node.(type) {
	case *List:
		d.writef(" %s\n", strconv.Quote(node.Marker))
		for _, item := range node.Items {
			d.node(item, depth+1)
		}
	case *ListItem:
		d.writef(" %s\n", d.quote(node.Principal))
		d.nodes(node.Children, depth+1)
	case *Container:
		d.writef(" %s\n", d.quote(node.Delim.Open))
		d.nodes(node.Children, depth+1)
	case *Header:
		d.writef(" %s title %s\n", d.quote(node.Lines()), d.quote(node.Title))
	case *Heading:
		d.writef(" %s title %s\n", d.quote(node.Lines()), d.quote(node.Title))
	case *Setext:
		d.writef(" %s title %s\n", d.quote(node.Lines()), d.quote(node.Title))
	default:
		d.writef(" %s\n", d.quote(node.Lines()))
	}
}

func (d *dumper) quote(span Span) string {
	return strconv.Quote(string(d.doc.Src[span.Start:span.End]))
}

// fenceOf reports the delimiter of a delimited node. The three types carrying
// one share no accessor, since the printer tells them apart anyway.
func fenceOf(node Node) (Delimiter, bool) {
	switch node := node.(type) {
	case *Verbatim:
		return node.Delim, true
	case *Container:
		return node.Delim, true
	case *Table:
		return node.Delim, true
	}
	return Delimiter{}, false
}
