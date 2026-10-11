package block

import (
	"testing"

	"github.com/ypfaff/adocfmt/internal/tree"
)

// checkPartition guards what the package documentation promises: the tree
// partitions its source. A byte no span owns disappears when printed, and a
// byte two spans own is printed twice. It walks the tree in the order the
// printer emits it and fails at the first span that does not start where the
// previous one ended.
func checkPartition(t *testing.T, doc *tree.Document) {
	t.Helper()

	w := &walker{t: t}
	if !doc.BOM.Empty() {
		w.span("BOM", doc.BOM)
	}
	w.nodes(doc.Nodes, doc.Tail, len(doc.Src))
}

type walker struct {
	t  *testing.T
	at int
}

func (w *walker) span(what string, s tree.Span) {
	w.t.Helper()
	if s.Start != w.at {
		w.t.Fatalf("%s starts at byte %d, want %d", what, s.Start, w.at)
	}
	if s.End < s.Start {
		w.t.Fatalf("%s ends at byte %d, before its start %d", what, s.End, s.Start)
	}
	w.at = s.End
}

func (w *walker) reached(what string, end int) {
	w.t.Helper()
	if w.at != end {
		w.t.Fatalf("%s ends at byte %d, want %d", what, w.at, end)
	}
}

func (w *walker) nodes(nodes []tree.Node, tail tree.Gap, end int) {
	w.t.Helper()
	for _, node := range nodes {
		w.node(node)
	}
	w.span("tail gap", tail.Span)
	w.reached("node sequence", end)
}

func (w *walker) node(node tree.Node) {
	w.t.Helper()
	for _, meta := range node.Base().Meta {
		w.span("metadata gap", meta.Gap.Span)
		w.span("metadata", meta.Lines)
	}
	w.span("gap", node.Base().Gap.Span)

	switch node := node.(type) {
	case *tree.Container:
		w.span("opening delimiter", node.Delim.Open)
		if node.Delim.Closed() {
			w.nodes(node.Children, node.Tail, node.Delim.Close.Start)
			w.span("closing delimiter", node.Delim.Close)
		} else {
			w.nodes(node.Children, node.Tail, node.Lines.End)
		}
	case *tree.List:
		for _, item := range node.Items {
			w.node(item)
		}
	case *tree.ListItem:
		if node.Marker.Start < node.Principal.Start || node.Marker.End > node.Principal.End {
			w.t.Fatalf("marker %v lies outside its principal %v", node.Marker, node.Principal)
		}
		w.span("principal", node.Principal)
		for _, child := range node.Children {
			w.node(child)
		}
	default:
		w.span("lines", node.Base().Lines)
	}
	w.reached("lines", node.Base().Lines.End)
	w.reached("extent", node.Base().Extent().End)
}
