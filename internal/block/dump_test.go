package block

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ypfaff/adocfmt/internal/tree"
)

// dump renders the tree as one line per node, so a test states the structure it
// expects instead of reaching into fields.
func dump(doc *tree.Document) string {
	d := dumper{doc: doc}
	d.nodes(doc.Nodes, 0)
	return d.out.String()
}

type dumper struct {
	doc *tree.Document
	out strings.Builder
}

// writef keeps the one call that could report an error, and never does, in a
// single place.
func (d *dumper) writef(format string, args ...any) {
	_, _ = fmt.Fprintf(&d.out, format, args...)
}

func (d *dumper) nodes(nodes []tree.Node, depth int) {
	for _, node := range nodes {
		d.node(node, depth)
	}
}

func (d *dumper) node(node tree.Node, depth int) {
	d.writef("%s%s", strings.Repeat("  ", depth), reflect.TypeOf(node).Elem().Name())
	if node.Base().Frozen {
		d.writef("!")
	}
	if mark := stageMark(node.Base().Gap); mark != "" {
		d.writef(" gap%s", mark)
	}
	// A bang on the fence marks a body a directive may extend, the way it marks
	// a frozen node and an exact gap. A list has no fence to carry it.
	if delim, ok := fenceOf(node); ok && delim.Extensible {
		d.writef(" delim!")
	}
	if list, ok := node.(*tree.List); ok && list.Extensible {
		d.writef(" extensible!")
	}
	for _, meta := range node.Base().Meta {
		// The mark is the stage of the gap above the metadata line.
		d.writef(" meta%s(%s)", stageMark(meta.Gap), d.quote(meta.Lines))
	}

	switch node := node.(type) {
	case *tree.List:
		d.writef(" %s\n", strconv.Quote(node.Marker))
		for _, item := range node.Items {
			d.node(item, depth+1)
		}
	case *tree.ListItem:
		d.writef(" %s\n", d.quote(node.Principal))
		d.nodes(node.Children, depth+1)
	case *tree.Container:
		d.writef(" %s\n", d.quote(node.Delim.Open))
		d.nodes(node.Children, depth+1)
	case *tree.Header:
		d.writef(" %s title %s\n", d.quote(node.Lines), d.quote(node.Title))
	case *tree.Heading:
		d.writef(" %s title %s\n", d.quote(node.Lines), d.quote(node.Title))
	case *tree.Setext:
		d.writef(" %s title %s\n", d.quote(node.Lines), d.quote(node.Title))
	default:
		d.writef(" %s\n", d.quote(node.Base().Lines))
	}
}

// stageMark marks a gap that is not free: ~ one that keeps its count, ! an
// exact one.
func stageMark(gap tree.Gap) string {
	switch gap.Stage {
	case tree.KeepCount:
		return "~"
	case tree.Exact:
		return "!"
	default:
		return ""
	}
}

func (d *dumper) quote(span tree.Span) string {
	return strconv.Quote(string(d.doc.Src[span.Start:span.End]))
}

// fenceOf reports the delimiter of a delimited node. The three types carrying
// one share no accessor, since the printer tells them apart anyway.
func fenceOf(node tree.Node) (tree.Delimiter, bool) {
	switch node := node.(type) {
	case *tree.Verbatim:
		return node.Delim, true
	case *tree.Container:
		return node.Delim, true
	case *tree.Table:
		return node.Delim, true
	}
	return tree.Delimiter{}, false
}
