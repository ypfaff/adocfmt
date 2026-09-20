package block

import (
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "the document header stays one node",
			src:  "= Title\nAuthor Name\n:toc:\n\nText.\n",
			want: `
Header "= Title\nAuthor Name\n:toc:\n"
Paragraph "Text.\n"`,
		},
		{
			name: "attribute entries and conditionals before the title do not end the header",
			src:  ":a: b\n\nifdef::x[]\n:c: d\nendif::[]\n= Title\nAuthor Name\n\nText.\n",
			want: `
Attribute ":a: b\n"
Directive! gap! "ifdef::x[]\n"
Attribute gap! ":c: d\n"
Directive! gap! "endif::[]\n"
Header gap! "= Title\nAuthor Name\n"
Paragraph "Text.\n"`,
		},
		{
			name: "a two-line document title is the header too",
			src:  "Reference Guide\n===============\n:toc:\nDan Allen\n\npreamble\n",
			want: `
Header "Reference Guide\n===============\n:toc:\nDan Allen\n"
Paragraph "preamble\n"`,
		},
		{
			name: "a title and its underline beat the listing delimiter",
			src:  "Mein Titel\n----------\n\nText.\n",
			want: `
Setext "Mein Titel\n----------\n" title "Mein Titel"
Paragraph "Text.\n"`,
		},
		{
			name: "the underline is measured in characters",
			src:  "Größe\n-----\n",
			want: `
Setext "Größe\n-----\n" title "Größe"`,
		},
		{
			name: "an underline two characters off is a delimiter",
			src:  "Titel\n-------\n\nText.\n",
			want: `
Paragraph "Titel\n"
Verbatim "-------\n\nText.\n"`,
		},
		{
			name: "delimiters pair by exact width",
			src:  "-----\ncode\n----\nstill code\n-----\n",
			want: `
Verbatim "-----\ncode\n----\nstill code\n-----\n"`,
		},
		{
			name: "a fence may name a language, and only the bare fence closes it",
			src:  "```ruby,numbered\nputs 1\n```ruby\n```\n\n````\nprose\n````\n",
			want: `
Verbatim "` + "```" + `ruby,numbered\nputs 1\n` + "```" + `ruby\n` + "```" + `\n"
Paragraph "` + "````" + `\nprose\n` + "````" + `\n"`,
		},
		{
			name: "an example block holds nodes, a listing block does not",
			src:  "====\nFirst.\n\nSecond.\n====\n",
			want: `
Container "====\n"
  Paragraph "First.\n"
  Paragraph "Second.\n"`,
		},
		{
			name: "below section level a heading line is prose",
			src:  "====\n== H ==\nText.\n====\n\n* a\n+\n== H\n* b\n",
			want: `
Container "====\n"
  Paragraph "== H ==\nText.\n"
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph "== H\n"
  ListItem "* b\n"`,
		},
		{
			name: "and a title with an underline is prose above a delimiter",
			src:  "====\nOuter.\n======\nInner.\n======\n====\n",
			want: `
Container "====\n"
  Paragraph "Outer.\n"
  Container "======\n"
    Paragraph "Inner.\n"`,
		},
		{
			name: "unless the heading is discrete",
			src:  "====\n[discrete]\n== H\n\nText.\n====\n",
			want: `
Container "====\n"
  Heading meta("[discrete]\n") "== H\n" title "H"
  Paragraph "Text.\n"`,
		},
		{
			name: "an attribute line turns prose into code",
			src:  "[source,go]\nx := 1\n",
			want: `
Literal meta("[source,go]\n") "x := 1\n"`,
		},
		{
			name: "metadata binds across a blank line",
			src:  "[#id]\n\nText.\n",
			want: `
Paragraph meta("[#id]\n") "Text.\n"`,
		},
		{
			name: "a table stays opaque",
			src:  "|===\n| a | b\n|===\n",
			want: `
Table "|===\n| a | b\n|===\n"`,
		},
		{
			name: "a change of marker nests, the old marker returns",
			src:  "* a\n- b\n* c\n",
			want: `
List "*"
  ListItem "* a\n"
    List "-"
      ListItem "- b\n"
  ListItem "* c\n"`,
		},
		{
			name: "a continuation carries the block that follows it",
			src:  "* a\n+\n----\ncode\n----\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Verbatim "----\ncode\n----\n"
  ListItem "* b\n"`,
		},
		{
			name: "a continuation at the end carries nothing and keeps the blank lines",
			src:  "* a\n+\n\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"`,
		},
		{
			name: "an indented block after a blank line attaches to the item",
			src:  "* a\n\n  code\n\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal "  code\n"
  ListItem "* b\n"`,
		},
		{
			name: "a directive freezes its own gap and the next one",
			src:  "Text.\n\ninclude::part.adoc[]\n\nMore.\n",
			want: `
Paragraph "Text.\n"
Directive! gap! "include::part.adoc[]\n"
Paragraph gap! "More.\n"`,
		},
		{
			name: "a directive inside a paragraph freezes the paragraph",
			src:  "First line.\ninclude::part.adoc[]\nThird line.\n",
			want: `
Paragraph! gap! "First line.\ninclude::part.adoc[]\nThird line.\n"`,
		},
		{
			name: "a directive between metadata and its block keeps them bound and freezes the block",
			src:  "[source]\ninclude::part.adoc[]\ncode\n\nAfter.\n",
			want: `
Literal! gap! meta("[source]\n") meta("include::part.adoc[]\n") "code\n"
Paragraph gap! "After.\n"`,
		},
		{
			name: "also inside a list, where the block stays attached to the item",
			src:  "* a\n+\n.Title\ninclude::part.adoc[]\nText.\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph! gap! meta(".Title\n") meta("include::part.adoc[]\n") "Text.\n"
  ListItem gap! "* b\n"`,
		},
		{
			name: "an anchor line and a lone + end a paragraph",
			src:  "Text.\n[[id]]\nMore.\n\nnormal text\n+\n",
			want: `
Paragraph "Text.\n"
Paragraph meta("[[id]]\n") "More.\n"
Paragraph "normal text\n"
Continuation gap! "+\n"`,
		},
		{
			name: "a lone + ends the paragraph a continuation carries as well",
			src:  "* a\n+\nparagraph two\n+\n----\ncode\n----\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph "paragraph two\n"
    Continuation gap! "+\n"
    Verbatim "----\ncode\n----\n"`,
		},
		{
			name: "a thematic break is not prose",
			src:  "Text.\n\n---\n\nMore.\n",
			want: `
Paragraph "Text.\n"
Opaque "---\n"
Paragraph "More.\n"`,
		},
		{
			name: "so is an evenly spaced one, and a page break",
			src:  "* * *\n\n<<<\n",
			want: `
Opaque "* * *\n"
Opaque "<<<\n"`,
		},
		{
			name: "four of the same char is a delimiter, not a break",
			src:  "****\nAside.\n****\n",
			want: `
Container "****\n"
  Paragraph "Aside.\n"`,
		},
		{
			name: "a break only counts on its own line",
			src:  "Text.\n---\nMore.\n",
			want: `
Paragraph "Text.\n---\nMore.\n"`,
		},
		{
			name: "Asciidoctor reads a Markdown heading as a section title",
			src:  "## Section One\n\nText.\n",
			want: `
Heading "## Section One\n" title "Section One"
Paragraph "Text.\n"`,
		},
		{
			name: "the block macros that need no extension are not prose",
			src:  "image::tiger.png[Tiger]\n\ntoc::[]\n\nfoo::bar[]\n",
			want: `
Opaque "image::tiger.png[Tiger]\n"
Opaque "toc::[]\n"
Paragraph "foo::bar[]\n"`,
		},
		{
			name: "a Markdown quote is a quote block, not a paragraph",
			src:  "> A famous quote.\n> -- Famous Person\n",
			want: `
Opaque "> A famous quote.\n> -- Famous Person\n"`,
		},
		{
			name: "a term with no text of its own takes the lines below it",
			src:  "term1::\n\n'''\ncontinued\n",
			want: `
List "::"
  ListItem "term1::\n\n'''\ncontinued\n"`,
		},
		{
			name: "inside a list a paragraph ends at the next item",
			src:  "term1::\n\ndef1\nterm2::\n\ndef2\n",
			want: `
List "::"
  ListItem "term1::\n\ndef1\n"
  ListItem "term2::\n\ndef2\n"`,
		},
		{
			name: "trailing whitespace does not hide a delimiter",
			src:  "----  \ncode\n----\n\nAfter.\n",
			want: `
Verbatim "----  \ncode\n----\n"
Paragraph "After.\n"`,
		},
		{
			name: "nor an attribute line or a directive",
			src:  "[source]  \ncode\n\ninclude::part.adoc[]  \n",
			want: `
Literal meta("[source]  \n") "code\n"
Directive! gap! "include::part.adoc[]  \n"`,
		},
		{
			name: "nor a continuation",
			src:  "* a\n+  \n----\ncode\n----\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+  \n"
    Verbatim "----\ncode\n----\n"`,
		},
		{
			name: "a heading title ends before trailing whitespace and the closing marker",
			src:  "== Title ==  \n",
			want: `
Heading "== Title ==  \n" title "Title"`,
		},
		{
			name: "a setext title is measured without trailing whitespace",
			src:  "Title  \n-----\n",
			want: `
Setext "Title  \n-----\n" title "Title"`,
		},
		{
			name: "front matter is not AsciiDoc",
			src:  "---\ntitle: x\n---\n\n= Title\n",
			want: `
FrontMatter "---\ntitle: x\n---\n"
Header "= Title\n"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			doc, err := Scan([]byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			checkPartition(t, doc)
			if got, want := dump(doc), strings.TrimPrefix(test.want, "\n")+"\n"; got != want {
				t.Errorf("tree differs\ngot:\n%swant:\n%s", got, want)
			}
		})
	}
}

func TestScanReports(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want Finding
	}{
		{
			name: "a block that never closes",
			src:  "----\ncode\n",
			want: Finding{Line: 1, Severity: Warn, Message: "block has no closing delimiter"},
		},
		{
			name: "a delimiter that closes outside the region it opened in",
			src:  "ifdef::extra[]\n----\nendif::[]\ncode\n----\n",
			want: Finding{Line: 2, Severity: Skip, Message: "delimiter opens and closes in different conditional regions"},
		},
		{
			name: "also when the conditional sits among the metadata lines",
			src:  "[source]\nifdef::extra[]\n----\nendif::[]\ncode\n----\n",
			want: Finding{Line: 3, Severity: Skip, Message: "delimiter opens and closes in different conditional regions"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			doc, err := Scan([]byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Findings) != 1 || doc.Findings[0] != test.want {
				t.Errorf("got %+v, want exactly %+v", doc.Findings, test.want)
			}
		})
	}
}

// TestScanRejects covers what the scanner must not repair silently.
func TestScanRejects(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "mixed line endings", src: "a\r\nb\n"},
		{name: "a carriage return ending no line", src: "a\rb\n"},
		{name: "not UTF-8", src: "a\xffb\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Scan([]byte(test.src)); err == nil {
				t.Error("got no error, want one")
			}
		})
	}
}
