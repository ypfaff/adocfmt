package printer

import (
	"bytes"
	"testing"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/testdocs"
)

// FuzzPrintIsIdentity searches for source the tree does not partition. The
// seeds are the line shapes classify tells apart, in the arrangements that
// make it backtrack: a list continuation with nothing to carry lost its blank
// lines this way. Every document TestPrintIsIdentity reads is a seed too, so
// the search also starts from shapes no seed here spells out.
//
//	go test -fuzz=FuzzPrintIsIdentity ./internal/printer
func FuzzPrintIsIdentity(f *testing.F) {
	for _, seed := range []string{
		"",
		"\n\n",
		"\xef\xbb\xbf= Title\n",
		"= Title\nAuthor\n:toc:\n\nText.\n",
		"Title\n-----\n\nText.\n",
		"Größe\n-----\n",
		"== Section ==\n\n# Markdown\n",
		"----\ncode\n\n----\n",
		"-----\ncode\n----\nmore\n-----\n",
		"====\nFirst.\n\nSecond.\n====\n",
		"====\n======\nInner.\n======\n====\n",
		"--\nopen\n--\n",
		"```\nfenced\n```\n",
		"|===\n| a | b\n\n| c | d\n|===\n",
		"[source,go]\nx := 1\n",
		"[#id]\n\n.Title\n// comment\nText.\n",
		"[[anchor]]\nText.\n",
		":name: value\n:!unset:\n",
		"* a\n- b\n* c\n",
		"* a\n+\n----\ncode\n----\n* b\n",
		"* a\n+\n\n",
		"* a\n\n  literal\n\n* b\n",
		"term::\n\n'''\ncontinued\n",
		"term1:: def\nterm2::\ndef\n",
		". one\n. two\n1. three\n",
		"<1> callout\n<.> next\n",
		"Text.\n\ninclude::part.adoc[]\n\nMore.\n",
		"First.\ninclude::part.adoc[]\nThird.\n",
		"ifdef::x[]\n----\nendif::[]\ncode\n----\n",
		"ifdef::x[]\n====\ntext\n====\nendif::[]\n",
		"Text.\n\n---\n\n* * *\n\n<<<\n",
		"image::a.png[]\n\ntoc::[]\n",
		"> quote\n> -- Someone\n",
		"---\ntitle: x\n---\n\n= Title\n",
		"no newline at the end",
		"a\r\nb\r\n",
		"  indented\n\ttabbed\n",
		"trailing.  \nbreak +  \n \t\n",
	} {
		f.Add([]byte(seed))
	}
	for _, doc := range testdocs.Documents(f) {
		f.Add(doc.Src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		doc, err := block.Parse(src)
		if err != nil {
			return
		}
		if got := PrintRaw(doc); !bytes.Equal(got, src) {
			t.Errorf("output differs from the source at byte %d", firstDifference(got, src))
		}
	})
}
