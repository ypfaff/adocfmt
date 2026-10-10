package block

import (
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "the document header stays one node",
			src:  "= Title\nAuthor Name\n:toc:\n\nText.\n",
			want: `
Header "= Title\nAuthor Name\n:toc:\n" title "Title"
Paragraph "Text.\n"`,
		},
		{
			name: "the header takes the author and the revision line and no more",
			src:  "= Title\nAuthor Name\nv1, 2020\n- one\n- two\n",
			want: `
Header "= Title\nAuthor Name\nv1, 2020\n" title "Title"
List "-"
  ListItem "- one\n"
  ListItem "- two\n"`,
		},
		{
			name: "a line starting with a colon is no revision, so the header ends at the author line",
			src:  "= Title\nAuthor Name\n:x\nText.\n",
			want: `
Header "= Title\nAuthor Name\n" title "Title"
Paragraph ":x\nText.\n"`,
		},
		{
			name: "a field after a comma that starts otherwise makes it a revision",
			src:  "= Title\nAuthor Name\n:x, 2020\n",
			want: `
Header "= Title\nAuthor Name\n:x, 2020\n" title "Title"`,
		},
		{
			name: "a comment line under the title is read without being counted",
			src:  "= Title\nAuthor Name\n// note\nv1, 2020\n- one\n",
			want: `
Header "= Title\nAuthor Name\n// note\nv1, 2020\n" title "Title"
List "-"
  ListItem "- one\n"`,
		},
		{
			name: "a directive under the title is not counted either, and freezes the header",
			src:  "= Title\nAuthor Name\nifdef::env-github[]\n:tip-caption: :bulb:\nendif::[]\nv1, 2020\n\nText.\n",
			want: `
Header! gap! "= Title\nAuthor Name\nifdef::env-github[]\n:tip-caption: :bulb:\nendif::[]\nv1, 2020\n" title "Title"
Paragraph gap! "Text.\n"`,
		},
		{
			name: "a frozen header runs to the blank line, since which lines it counts is open",
			src:  "= Title\nifdef::x[]\nJane Doe\nendif::[]\nifndef::x[]\nJohn Roe\nendif::[]\nv1, 2020\n\nText.\n",
			want: `
Header! gap! "= Title\nifdef::x[]\nJane Doe\nendif::[]\nifndef::x[]\nJohn Roe\nendif::[]\nv1, 2020\n" title "Title"
Paragraph gap! "Text.\n"`,
		},
		{
			name: "attribute entries and conditionals before the title do not end the header",
			src:  ":a: b\n\nifdef::x[]\n:c: d\nendif::[]\n= Title\nAuthor Name\n\nText.\n",
			want: `
Attribute ":a: b\n"
Directive! gap! "ifdef::x[]\n"
Attribute! gap! ":c: d\n"
Directive! gap! "endif::[]\n"
Header! gap! "= Title\nAuthor Name\n" title "Title"
Paragraph gap! "Text.\n"`,
		},
		{
			name: "a comment block before the title does not end the header either",
			src:  "////\nlicense\n////\n= Title\nAuthor Name\n\nText.\n",
			want: `
Verbatim "////\nlicense\n////\n"
Header "= Title\nAuthor Name\n" title "Title"
Paragraph "Text.\n"`,
		},
		{
			name: "a comment block under the title is read without being counted",
			src:  "= Title\n////\nnote\n////\nAuthor Name\nv1, 2020\n\nText.\n",
			want: `
Header "= Title\n////\nnote\n////\nAuthor Name\nv1, 2020\n" title "Title"
Paragraph "Text.\n"`,
		},
		{
			name: "an attribute name may start with any character Ruby counts as a word character",
			src:  ":Ⓐ-one: first\n:Ⓐ-two: second\n",
			want: `
Attribute ":Ⓐ-one: first\n"
Attribute ":Ⓐ-two: second\n"`,
		},
		{
			name: "a backslash after a space continues the value on the next line",
			src:  ":a: one \\\ntwo\n\nText.\n",
			want: `
Attribute ":a: one \\\ntwo\n"
Paragraph "Text.\n"`,
		},
		{
			name: "a plus after a space continues the value too",
			src:  ":a: one +\ntwo\n\nText.\n",
			want: `
Attribute ":a: one +\ntwo\n"
Paragraph "Text.\n"`,
		},
		{
			name: "the value runs while lines end in its own marker, and a blank line ends it",
			src:  ":a: one \\\ntwo \\\nthree +\nfour\n\n:b: one \\\n\ntwo\n",
			want: `
Attribute ":a: one \\\ntwo \\\nthree +\n"
Paragraph "four\n"
Attribute ":b: one \\\n"
Paragraph "two\n"`,
		},
		{
			name: "a continuation line is the value whatever it looks like",
			src:  ":a: one \\\n* item\n:b: c\n",
			want: `
Attribute ":a: one \\\n* item\n"
Attribute ":b: c\n"`,
		},
		{
			name: "a directive among the continuation lines freezes the entry",
			src:  ":a: one \\\nifdef::x[]\ntwo\nendif::[]\n\nText.\n",
			want: `
Attribute! gap! ":a: one \\\nifdef::x[]\ntwo\n"
Directive! gap! "endif::[]\n"
Paragraph gap! "Text.\n"`,
		},
		{
			name: "in the header a continuation line is no entry of its own",
			src:  "= Title\n:a: one \\\n:hardbreaks:\n\na\nb\n",
			want: `
Header "= Title\n:a: one \\\n:hardbreaks:\n" title "Title"
Paragraph "a\nb\n"`,
		},
		{
			name: "a name may hold spaces, dots and slashes, and is sanitized before it binds",
			src:  ":a b: v\n:a.b: v\n:a/b: v\n:Hard Breaks:\n\na\nb\n",
			want: `
Attribute ":a b: v\n"
Attribute ":a.b: v\n"
Attribute ":a/b: v\n"
Attribute ":Hard Breaks:\n"
Literal "a\nb\n"`,
		},
		{
			name: "a colon glued to the value is prose, a doubled one a term",
			src:  ":name:value\nText.\n\n:a:: b\nText.\n",
			want: `
Paragraph ":name:value\nText.\n"
List "::"
  ListItem ":a:: b\nText.\n"`,
		},
		{
			name: "a two-line document title is the header too",
			src:  "Reference Guide\n===============\n:toc:\nDan Allen\n\npreamble\n",
			want: `
Header "Reference Guide\n===============\n:toc:\nDan Allen\n" title "Reference Guide"
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
			name: "a directive in the body marks the fence, since what it brings in may close the block",
			src:  "----\ninclude::part.adoc[]\n----\n\n====\nText.\n\nifdef::x[]\nMore.\nendif::[]\n====\n",
			want: `
Verbatim delim! "----\ninclude::part.adoc[]\n----\n"
Container delim! "====\n"
  Paragraph "Text.\n"
  Directive! gap! "ifdef::x[]\n"
  Paragraph! gap! "More.\nendif::[]\n"`,
		},
		{
			name: "a directive nested deeper marks the fence around it too",
			src:  "====\n----\ninclude::part.adoc[]\n----\n====\n",
			want: `
Container delim! "====\n"
  Verbatim delim! "----\ninclude::part.adoc[]\n----\n"`,
		},
		{
			name: "a comment block is never extensible, and neither is the open block a [comment] makes one",
			src:  "////\ninclude::part.adoc[]\n////\n\n[comment]\n--\ninclude::part.adoc[]\n--\n",
			want: `
Verbatim "////\ninclude::part.adoc[]\n////\n"
Verbatim meta("[comment]\n") "--\ninclude::part.adoc[]\n--\n"`,
		},
		{
			name: "a [comment] on a wider fence is dropped, so a directive in the body extends it all the same",
			src:  "[comment]\n----\ninclude::part.adoc[]\n----\n",
			want: `
Verbatim delim! meta("[comment]\n") "----\ninclude::part.adoc[]\n----\n"`,
		},
		{
			name: "a directive above a block freezes the block rather than marking its fence",
			src:  "include::part.adoc[]\n----\ncode\n----\n",
			want: `
Directive! gap! "include::part.adoc[]\n"
Verbatim! gap! "----\ncode\n----\n"`,
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
			name: "a named attribute is not a style, a quoted one is",
			src:  "[source]\n[role=x]\ncode\n\n[role=x]\nText.\n\n[\"source\"]\ncode\n",
			want: `
Literal meta("[source]\n") meta("[role=x]\n") "code\n"
Paragraph meta("[role=x]\n") "Text.\n"
Literal meta("[\"source\"]\n") "code\n"`,
		},
		{
			name: "a verbatim style reads an item line, an indent and a delimiter as code",
			src:  "[source]\n* a\n* b\n\n[listing]\n  indented\n----\nmore\n----\n\n[verse]\n> quoted\n'''\n\nText.\n",
			want: `
Literal meta("[source]\n") "* a\n* b\n"
Literal meta("[listing]\n") "  indented\n----\nmore\n----\n"
Literal meta("[verse]\n") "> quoted\n'''\n"
Paragraph "Text.\n"`,
		},
		{
			name: "a verbatim style ends at a lone plus, and a delimiter below it still opens a block",
			src:  "[source]\ncode\n+\nText.\n\n[source]\n----\n* a\n----\n",
			want: `
Literal meta("[source]\n") "code\n"
Paragraph! gap! "+\nText.\n"
Verbatim meta("[source]\n") "----\n* a\n----\n"`,
		},
		{
			name: "but a lone plus on its first line is code, and a delimiter under it too",
			src:  "[source]\n+\n////\nc\n////\n\n[literal]\n+\n+\nText.\n",
			want: `
Literal meta("[source]\n") "+\n////\nc\n////\n"
Literal meta("[literal]\n") "+\n"
Paragraph! gap! "+\nText.\n"`,
		},
		{
			name: "a pass style still lets the line decide",
			src:  "[pass]\n* a\n* b\n",
			want: `
List meta("[pass]\n") "*"
  ListItem "* a\n"
  ListItem "* b\n"`,
		},
		{
			name: "the hardbreaks option makes the line breaks content",
			src:  "[%hardbreaks]\nread\nmy\nlips\n\n[quote%hardbreaks]\na\nb\n\n[.lead]\n[opts=hardbreaks]\na\nb\n\n[role=hardbreaks]\na\nb\n",
			want: `
Literal meta("[%hardbreaks]\n") "read\nmy\nlips\n"
Literal meta("[quote%hardbreaks]\n") "a\nb\n"
Literal meta("[.lead]\n") meta("[opts=hardbreaks]\n") "a\nb\n"
Paragraph meta("[role=hardbreaks]\n") "a\nb\n"`,
		},
		{
			name: "the option on a container does not reach its paragraphs",
			src:  "[%hardbreaks]\n--\na\nb\n--\n",
			want: `
Container meta("[%hardbreaks]\n") "--\n"
  Paragraph "a\nb\n"`,
		},
		{
			name: "hardbreaks-option in the header holds for the document",
			src:  "= Title\n:hardbreaks-option:\n\na\nb\n\n* item\n+\nc\nd\n\n====\ne\nf\n====\n",
			want: `
Header "= Title\n:hardbreaks-option:\n" title "Title"
Literal "a\nb\n"
List "*"
  ListItem "* item\n"
    Continuation gap! "+\n"
    Literal "c\nd\n"
Container "====\n"
  Literal "e\nf\n"`,
		},
		{
			name: "an entry binds lines from where it stands until one unsets it",
			src:  "a\nb\n\n:hardbreaks:\n\nc\nd\n\n:!hardbreaks-option:\n\ne\nf\n",
			want: `
Paragraph "a\nb\n"
Attribute ":hardbreaks:\n"
Literal "c\nd\n"
Attribute ":!hardbreaks-option:\n"
Paragraph "e\nf\n"`,
		},
		{
			name: "attribute-missing drop-line binds lines, any other value frees them",
			src:  ":attribute-missing: drop-line\n\na {x}\nb\n\n:attribute-missing: skip\n\nc\nd\n\n:attribute-missing: drop-line\n\n:attribute-missing!:\n\ne\nf\n",
			want: `
Attribute ":attribute-missing: drop-line\n"
Literal "a {x}\nb\n"
Attribute ":attribute-missing: skip\n"
Paragraph "c\nd\n"
Attribute ":attribute-missing: drop-line\n"
Attribute ":attribute-missing!:\n"
Paragraph "e\nf\n"`,
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
			name: "ordered items share a list by marker style",
			src:  "1. a\n2. b\n",
			want: `
List "1."
  ListItem "1. a\n"
  ListItem "2. b\n"`,
		},
		{
			name: "a letter and a dot is an ordered marker, upper and lower case apart",
			src:  "a. a\nA. b\nb. c\n",
			want: `
List "a."
  ListItem "a. a\n"
    List "A."
      ListItem "A. b\n"
  ListItem "b. c\n"`,
		},
		{
			name: "roman numerals with a parenthesis are one too",
			src:  "i) a\nii) b\nIII) c\niv) d\n",
			want: `
List "i)"
  ListItem "i) a\n"
  ListItem "ii) b\n"
    List "I)"
      ListItem "III) c\n"
  ListItem "iv) d\n"`,
		},
		{
			name: "a change of ordered marker style nests, like any change of marker",
			src:  "1. a\na. b\n2. c\n",
			want: `
List "1."
  ListItem "1. a\n"
    List "a."
      ListItem "a. b\n"
  ListItem "2. c\n"`,
		},
		{
			name: "the bullet character is an unordered marker of its own",
			src:  "• a\n* b\n• c\n",
			want: `
List "•"
  ListItem "• a\n"
    List "*"
      ListItem "* b\n"
  ListItem "• c\n"`,
		},
		{
			name: "a marker with no text after it is prose",
			src:  "*\nText.\n\n1.\nText.\n\n<1>\nText.\n\n•\nText.\n",
			want: `
Paragraph "*\nText.\n"
Paragraph "1.\nText.\n"
Paragraph "<1>\nText.\n"
Paragraph "•\nText.\n"`,
		},
		{
			name: "also inside a list, where it is text of the item above",
			src:  "* a\n*\n* b\n",
			want: `
List "*"
  ListItem "* a\n*\n"
  ListItem "* b\n"`,
		},
		{
			name: "a tab after the marker counts as space",
			src:  "*\ta\n1.\tb\n",
			want: `
List "*"
  ListItem "*\ta\n"
    List "1."
      ListItem "1.\tb\n"`,
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
			name: "past an attribute entry it carries the block below",
			src:  "a::\n+\n:x: y\nmore\nb::: c\n",
			want: `
List "::"
  ListItem "a::\n"
    Continuation gap! "+\n"
    Attribute ":x: y\n"
    Paragraph! "more\nb::: c\n"`,
		},
		{
			name: "even across a blank line",
			src:  "* a\n+\n:x: y\n\nmore\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Attribute ":x: y\n"
    Paragraph "more\n"
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
  ListItem gap! "* b\n"`,
		},
		{
			name: "an indented callout is a literal block, not an item",
			src:  "<1> a\n\n <2> b\n",
			want: `
List "<>"
  ListItem "<1> a\n"
    Literal " <2> b\n"`,
		},
		{
			name: "a literal paragraph runs to the blank line, an item line included",
			src:  "* a\n\n  code\n* b\n\n* c\n\nText.\n\n  more\n* d\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal! "  code\n* b\n"
  ListItem gap! "* c\n"
Paragraph "Text.\n"
Literal "  more\n* d\n"`,
		},
		{
			name: "a literal paragraph in an item ends at a delimiter, and the block there belongs to the item",
			src:  "* a\n\n  code\n----\nx\n----\n\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal "  code\n"
    Verbatim "----\nx\n----\n"
  ListItem gap! "* b\n"`,
		},
		{
			name: "that block may close below a blank line, as the item reads on past one",
			src:  "* a\n\n  code\n----\nx\n\n  more\n----\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal "  code\n"
    Verbatim "----\nx\n\n  more\n----\n"`,
		},
		{
			name: "so does one under a continuation, and the next term ends it",
			src:  "a:: b\n+\n  code\n[source]\n----\nx\n----\nc:: d\n",
			want: `
List "::"
  ListItem "a:: b\n"
    Continuation gap! "+\n"
    Literal "  code\n"
    Verbatim meta("[source]\n") "----\nx\n----\n"
  ListItem "c:: d\n"`,
		},
		{
			name: "an item line under that block opens a list nested in the item, even one with the item's own marker",
			src:  "* a\n\n  lit\n----\nx\n----\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal "  lit\n"
    Verbatim "----\nx\n----\n"
    List "*"
      ListItem gap! "* b\n"`,
		},
		{
			name: "a term of a nested list ends the literal paragraph, while the item around the list takes the block",
			src:  "* a\nc:: d\n\n  code\ne:: f\n----\nx\n----\n",
			want: `
List "*"
  ListItem "* a\n"
    List "::"
      ListItem "c:: d\n"
        Literal "  code\n"
      ListItem gap! "e:: f\n"
    Verbatim "----\nx\n----\n"`,
		},
		{
			name: "no blank line may come between the blocks inside a delimited block there either",
			src:  "* a\n\n  lit\n====\ntext\n----\nx\n----\n====\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal "  lit\n"
    Container "====\n"
      Paragraph gap! "text\n"
      Verbatim gap! "----\nx\n----\n"`,
		},
		{
			name: "a comment under the continuation ends it, so the block stays out",
			src:  "* a\n+\n// c\n  code\n----\nx\n----\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Literal meta("// c\n") "  code\n"
Verbatim "----\nx\n----\n"`,
		},
		{
			name: "the blank line between metadata and a literal paragraph in an item stays, the item takes the paragraph whole",
			src:  "* a\n[x]\n\n  code\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal! gap! meta("[x]\n") "  code\n* b\n"`,
		},
		{
			name: "the blank line between metadata and an indented line in a delimited block in an item may go",
			src:  "* a\n+\n--\n[x]\n\n  code\n--\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Container "--\n"
      Literal meta("[x]\n") "  code\n"`,
		},
		{
			name: "a + under a blank line reads as a blank line in a nested item, so the block it carries belongs to the item around the list",
			src:  "* d\n** x\n\n+\n[source]\nc\n** u\n",
			want: `
List "*"
  ListItem "* d\n"
    List "**"
      ListItem "** x\n"
    Continuation gap! "+\n"
    Literal! meta("[source]\n") "c\n** u\n"`,
		},
		{
			name: "where that + stands between two items of the nested list, the item above takes it, so the next one follows",
			src:  "* a\n1. one\n\n+\n1. two\n",
			want: `
List "*"
  ListItem "* a\n"
    List "1."
      ListItem "1. one\n"
        Continuation gap! "+\n"
      ListItem "1. two\n"`,
		},
		{
			name: "the blank line under the lines of a literal paragraph stays before the next item, also where they read as a list",
			src:  "- d\n+\n  t:: u\nb::\n\n- d\n",
			want: `
List "-"
  ListItem "- d\n"
    Continuation gap! "+\n"
    List "::"
      ListItem "  t:: u\n"
      ListItem gap! "b::\n"
  ListItem gap! "- d\n"`,
		},
		{
			name: "a line with the separator of a description list is its next term, whatever else it reads as",
			src:  "t:: u\n[source]\n* a:: b\n----\nx\n",
			want: `
List "::"
  ListItem "t:: u\n"
Literal meta("[source]\n") "* a:: b\n----\nx\n"`,
		},
		{
			name: "the blank line between metadata and an indented item of a nested list may go",
			src:  "* a\n[horizontal]\n\n  b:: c\n",
			want: `
List "*"
  ListItem "* a\n"
    List meta("[horizontal]\n") "::"
      ListItem "  b:: c\n"`,
		},
		{
			name: "under a term without text, the indented line is the text and the block stays out",
			src:  "t::\n\n  code\n----\n----\n",
			want: `
List "::"
  ListItem "t::\n"
    Literal "  code\n"
Verbatim "----\n----\n"`,
		},
		{
			name: "under a term without text, a block attribute line after one blank line ends the list",
			src:  "t::\n\n[.r]\ntext\n\nu:: v\n",
			want: `
List "::"
  ListItem "t::\n"
Paragraph meta("[.r]\n") "text\n"
List "::"
  ListItem "u:: v\n"`,
		},
		{
			name: "under a term without text, a block attribute line after two blank lines is the term's text",
			src:  "t::\n\n\n[.r]\ntext\n\nu:: v\n",
			want: `
List "::"
  ListItem "t::\n"
    Paragraph meta("[.r]\n") "text\n"
  ListItem "u:: v\n"`,
		},
		{
			name: "under a term without text, a fence after two blank lines opens a block in the item",
			src:  "t::\n\n\n----\nx\n----\n",
			want: `
List "::"
  ListItem "t::\n"
    Verbatim gap! "----\nx\n"
Verbatim "----\n"`,
		},
		{
			name: "a callout list after metadata and a blank line ends the list, the item keeps the metadata",
			src:  "* a\n[[id]]\n\n<1> b\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque meta("[[id]]\n") ""
List "<>"
  ListItem "<1> b\n"`,
		},
		{
			name: "a nested callout list ends at a blank line with the item it stands in",
			src:  "* a\n<1> b\n\n<2> c\n",
			want: `
List "*"
  ListItem "* a\n"
    List "<>"
      ListItem "<1> b\n"
List "<>"
  ListItem "<2> c\n"`,
		},
		{
			name: "a nested callout list goes on after a blank line under a term without text",
			src:  "* a\n<1> b\nt::\n\n<2> c\n\n<3> d\n",
			want: `
List "*"
  ListItem "* a\n"
    List "<>"
      ListItem "<1> b\n"
        List "::"
          ListItem "t::\n"
      ListItem "<2> c\n"
List "<>"
  ListItem "<3> d\n"`,
		},
		{
			name: "under a term without text, a callout list after a blank line is the term's text",
			src:  "t::\n\n<1> b\n\n<2> c\n",
			want: `
List "::"
  ListItem "t::\n"
    List "<>"
      ListItem "<1> b\n"
List "<>"
  ListItem "<2> c\n"`,
		},
		{
			name: "block attribute lines one blank line under a description stay in it when a callout list follows",
			src:  "a:: b\n\n[x]\n\n<1> c\n\nText.\n\nd:: e\n\n[x]\n<1> f\n",
			want: `
List "::"
  ListItem "a:: b\n"
    Opaque meta("[x]\n") ""
List "<>"
  ListItem "<1> c\n"
Paragraph "Text.\n"
List "::"
  ListItem "d:: e\n"
    List meta("[x]\n") "<>"
      ListItem "<1> f\n"`,
		},
		{
			name: "under a term without text, block attribute lines stay in the item together with the callout list below them",
			src:  "t::\n\n[.r]\n\n<1> b\n\n<2> c\n",
			want: `
List "::"
  ListItem "t::\n"
    List meta("[.r]\n") "<>"
      ListItem "<1> b\n"
List "<>"
  ListItem "<2> c\n"`,
		},
		{
			name: "a two-line title whose title line reads as a list item freezes",
			src:  "a:: b\n\n[[id]]\n\n<1> c\n-----\n",
			want: `
List "::"
  ListItem "a:: b\n"
    Opaque meta("[[id]]\n") ""
Setext! "<1> c\n-----\n" title "<1> c"`,
		},
		{
			name: "a space in a conditional target or at the edge of an include target makes no directive",
			src:  "ifdef::a b[]\n\ninclude:: part.adoc[]\n",
			want: `
Opaque "ifdef::a b[]\n"
List "::"
  ListItem "include:: part.adoc[]\n"`,
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
			name: "and every block down to the next blank line, since what it brings in may continue them",
			src:  "include::part.adoc[]\nTitle\n~~~~~\nText. More.\n\nLast.\n",
			want: `
Directive! gap! "include::part.adoc[]\n"
Setext! gap! "Title\n~~~~~\n" title "Title"
Paragraph! gap! "Text. More.\n"
Paragraph gap! "Last.\n"`,
		},
		{
			name: "an endif reaches the block under it the same way",
			src:  "ifdef::x[]\n:a: b\nendif::[]\n## Sec ##\n",
			want: `
Directive! gap! "ifdef::x[]\n"
Attribute! gap! ":a: b\n"
Directive! gap! "endif::[]\n"
Heading! gap! "## Sec ##\n" title "Sec"`,
		},
		{
			name: "and so does a conditional that brings in a line of its own",
			src:  "ifdef::x[Text.]\nOne. Two.\n",
			want: `
Directive! gap! "ifdef::x[Text.]\n"
Paragraph! gap! "One. Two.\n"`,
		},
		{
			name: "a list it reaches freezes whole",
			src:  "include::part.adoc[]\n* a\n\n* b\n",
			want: `
Directive! gap! "include::part.adoc[]\n"
List! gap! "*"
  ListItem "* a\n"
  ListItem gap! "* b\n"`,
		},
		{
			name: "inside a container it freezes only the blocks it reaches",
			src:  "====\ninclude::part.adoc[]\nText.\n\nMore.\n====\n",
			want: `
Container delim! "====\n"
  Directive! gap! "include::part.adoc[]\n"
  Paragraph! gap! "Text.\n"
  Paragraph gap! "More.\n"`,
		},
		{
			name: "and inside an item too",
			src:  "* c\n+\ninclude::part.adoc[]\n** d\n",
			want: `
List extensible! "*"
  ListItem "* c\n"
    Continuation gap! "+\n"
    Directive! gap! "include::part.adoc[]\n"
    List! gap! "**"
      ListItem "** d\n"`,
		},
		{
			name: "a header it reaches freezes, but still ends under the revision line",
			src:  "ifdef::x[]\n:a: b\nendif::[]\n= Title\nJane Doe\nv1.0\n----\ncode\n\nmore\n----\n",
			want: `
Directive! gap! "ifdef::x[]\n"
Attribute! gap! ":a: b\n"
Directive! gap! "endif::[]\n"
Header! gap! "= Title\nJane Doe\nv1.0\n" title "Title"
Verbatim! gap! "----\ncode\n\nmore\n----\n"`,
		},
		{
			name: "a term it reaches freezes, but takes no lines across a blank line",
			src:  "a:: x\nifdef::q[]\nendif::[]\nb::\ntext\n\nPara. Two.\n",
			want: `
List extensible! "::"
  ListItem! "a:: x\nifdef::q[]\nendif::[]\n"
  ListItem! gap! "b::\ntext\n"
Paragraph gap! "Para. Two.\n"`,
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
			name: "a comment block between metadata and its block keeps them bound",
			src:  "[source]\n////\nc\n////\nx = 1\ny = 2\n\n.Title\n// note\n////\nc\n////\n\nText.\n",
			want: `
Literal meta("[source]\n") meta("////\nc\n////\n") "x = 1\ny = 2\n"
Paragraph meta(".Title\n") meta("// note\n") meta("////\nc\n////\n") "Text.\n"`,
		},
		{
			name: "an attribute entry between metadata and its block keeps them bound, and still binds",
			src:  "[source]\n:x: y\ntext\nmore\n\n.Title\n:a: one \\\ntwo\n\nText.\n\n[.lead]\n:hardbreaks:\na\nb\n",
			want: `
Literal meta("[source]\n") meta(":x: y\n") "text\nmore\n"
Paragraph meta(".Title\n") meta(":a: one \\\ntwo\n") "Text.\n"
Literal meta("[.lead]\n") meta(":hardbreaks:\n") "a\nb\n"`,
		},
		{
			name: "an entry among metadata lines reaching past its last line is marked open",
			src:  "[.lead]\n:a: one \\\n\nText.\n\n[.lead]\n:b: one \\\ntwo\n\nMore.\n",
			want: `
Paragraph meta("[.lead]\n") meta!(":a: one \\\n") "Text.\n"
Paragraph meta("[.lead]\n") meta(":b: one \\\ntwo\n") "More.\n"`,
		},
		{
			name: "a comment block or an attribute entry with no metadata above is a block of its own",
			src:  "////\nc\n////\n.Title\nText.\n\n:x: y\n[source]\ncode\n",
			want: `
Verbatim "////\nc\n////\n"
Paragraph meta(".Title\n") "Text.\n"
Attribute ":x: y\n"
Literal meta("[source]\n") "code\n"`,
		},
		{
			name: "also inside a list, where the block stays attached to the item",
			src:  "* a\n+\n.Title\ninclude::part.adoc[]\nText.\n* b\n",
			want: `
List extensible! "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph! gap! meta(".Title\n") meta("include::part.adoc[]\n") "Text.\n"
  ListItem! gap! "* b\n"`,
		},
		{
			name: "an anchor line and a lone + end a paragraph",
			src:  "Text.\n[[id]]\nMore.\n\nnormal text\n+\n",
			want: `
Paragraph "Text.\n"
Paragraph meta("[[id]]\n") "More.\n"
Paragraph "normal text\n"
Paragraph! gap! "+\n"`,
		},
		{
			name: "a bracket line Asciidoctor does not read as an anchor or attribute list is prose",
			src:  "[<foo>]\n\nText.\n\n[[a b]]\n\nMore.\n",
			want: `
Paragraph "[<foo>]\n"
Paragraph "Text.\n"
Paragraph "[[a b]]\n"
Paragraph "More.\n"`,
		},
		{
			name: "a second dot opens a block title",
			src:  "..Title\nText.\n",
			want: `
Paragraph meta("..Title\n") "Text.\n"`,
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
			name: "an attribute line right after an item binds to a block inside the item",
			src:  "* a\n[.x]\ntext\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Paragraph meta("[.x]\n") "text\n"
  ListItem "* b\n"`,
		},
		{
			name: "and to no block when the next item follows it",
			src:  "* a\n[.x]\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque meta("[.x]\n") ""
  ListItem "* b\n"`,
		},
		{
			name: "also across a blank line, which then belongs to the next item",
			src:  "* a\n[x]\n\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque meta("[x]\n") ""
  ListItem "* b\n"`,
		},
		{
			name: "a blank line after the attribute line ends the item, and the next item line is prose outside",
			src:  "* a\n[x]\n\ntext\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque meta("[x]\n") ""
Paragraph "text\n* b\n"`,
		},
		{
			name: "unless a nested list follows the blank line",
			src:  "* a\n[x]\n\n** b\n",
			want: `
List "*"
  ListItem "* a\n"
    List meta("[x]\n") "**"
      ListItem "** b\n"`,
		},
		{
			name: "a delimiter after the attribute line breaks the list and leaves the line behind",
			src:  "* a\n[x]\n----\ncode\n----\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque meta("[x]\n") ""
Verbatim "----\ncode\n----\n"
List "*"
  ListItem "* b\n"`,
		},
		{
			name: "an attribute line after a blank line starts a new list",
			src:  "* a\n\n[x]\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
List meta("[x]\n") "*"
  ListItem "* b\n"`,
		},
		{
			name: "prose right after a carried block stays in the item",
			src:  "* a\n+\n----\ncode\n----\ntext\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Verbatim "----\ncode\n----\n"
    Paragraph "text\n"
  ListItem "* b\n"`,
		},
		{
			name: "a carried paragraph reads on past a nested item line, but ends at an item of an open list",
			src:  "* a\n+\npara\n- dash\n* b\n** c\n+\n\nmore\n* d\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph! "para\n- dash\n"
  ListItem "* b\n"
    List "**"
      ListItem "** c\n"
        Continuation gap! "+\n"
        Paragraph "more\n"
  ListItem "* d\n"`,
		},
		{
			name: "a description term inside a paragraph freezes it, joined onto the line above it would make a list",
			src:  "* a\n+\npara\nterm:: x\n\nText.\nfoo:: bar\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph! "para\nterm:: x\n"
Paragraph! "Text.\nfoo:: bar\n"`,
		},
		{
			name: "inside a carried list, a paragraph right after the item text ends at any item line again",
			src:  "* a\n+\n- b\n[.x]\npara\n. c\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    List "-"
      ListItem "- b\n"
        Paragraph meta("[.x]\n") "para\n"
        List "."
          ListItem ". c\n"`,
		},
		{
			name: "an item of the list ends a continuation, which then carries nothing",
			src:  "* a\n+\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
  ListItem "* b\n"`,
		},
		{
			name: "also across a blank line",
			src:  "* a\n+\n\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
  ListItem "* b\n"`,
		},
		{
			name: "and when the item belongs to an enclosing list",
			src:  "* a\n** b\n+\n\n* c\n",
			want: `
List "*"
  ListItem "* a\n"
    List "**"
      ListItem "** b\n"
        Continuation gap! "+\n"
  ListItem "* c\n"`,
		},
		{
			name: "two blank lines under a continuation end the item, and stay, as one would carry the block below",
			src:  "* a\n+\n\n\nText.\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
Paragraph gap! "Text.\n"`,
		},
		{
			name: "metadata after the continuation binds to no block then either",
			src:  "* a\n+\n[x]\n* b\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Opaque meta("[x]\n") ""
  ListItem "* b\n"`,
		},
		{
			name: "a comment block under item metadata ends the item, as any delimiter does",
			src:  "* a\n[x]\n////\nc\n////\ntext\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque meta("[x]\n") ""
Verbatim "////\nc\n////\n"
Paragraph "text\n"`,
		},
		{
			name: "a continuation ends with the comment block it carries, so a delimiter under it ends the item",
			src:  "* a\n+\n[x]\n////\nc\n////\n----\nx\n----\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Opaque meta("[x]\n") meta("////\nc\n////\n") ""
Verbatim "----\nx\n----\n"`,
		},
		{
			name: "text under a comment block a + carries stays in the item, with the metadata bound to it",
			src:  "* a\n+\n[.r]\n////\nc\n////\ntext\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Paragraph meta("[.r]\n") meta("////\nc\n////\n") "text\n"`,
		},
		{
			name: "a comment line ends a +, so a blank line under it ends the item",
			src:  "* a\n+\n// c\n\ntext\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Opaque meta("// c\n") ""
Paragraph "text\n"`,
		},
		{
			name: "a block attribute line under a comment line under a + ends a description list",
			src:  "t:: d\n+\n// c\n[.r]\ntext\n",
			want: `
List "::"
  ListItem "t:: d\n"
    Continuation gap! "+\n"
    Opaque meta("// c\n") ""
Paragraph meta("[.r]\n") "text\n"`,
		},
		{
			name: "a block attribute line under a block title after a delimited block a + carries ends a description list",
			src:  "t:: d\n+\n----\nx\n----\n.T\n[x]\ntext\n",
			want: `
List "::"
  ListItem "t:: d\n"
    Continuation gap! "+\n"
    Verbatim "----\nx\n----\n"
    Opaque meta(".T\n") ""
Opaque meta("[x]\n") "text\n"`,
		},
		{
			name: "a delimited block in an item confines the list, so an item line inside it is prose",
			src:  "* a\n+\n====\ntext\n* b\n====\n",
			want: `
List "*"
  ListItem "* a\n"
    Continuation gap! "+\n"
    Container "====\n"
      Paragraph "text\n* b\n"`,
		},
		{
			name: "a comment line inside a paragraph freezes it, but not its gaps",
			src:  "First line.\n// a comment\nSecond line.\n\nMore.\n",
			want: `
Paragraph! "First line.\n// a comment\nSecond line.\n"
Paragraph "More.\n"`,
		},
		{
			name: "so does a line that must stay a line of its own",
			src:  "Text.\n\\include::x.adoc[]\nMore.\n\nOne.\nTwo {set:a!} gone.\nThree.\n",
			want: `
Paragraph! "Text.\n\\include::x.adoc[]\nMore.\n"
Paragraph! "One.\nTwo {set:a!} gone.\nThree.\n"`,
		},
		{
			name: "a third slash makes a line no comment",
			src:  "///\nText.\n\n///term:: More.\n",
			want: `
Paragraph "///\nText.\n"
List "::"
  ListItem "///term:: More.\n"`,
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
			name: "only a Markdown break may be indented",
			src:  " ---\n\n '''\n",
			want: `
Opaque " ---\n"
Literal " '''\n"`,
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
			name: "the block macros that need no extension are not prose, and any other stays as it is",
			src:  "image::tiger.png[Tiger]\n\ntoc::[]\n\nfoo::bar[]\n\ninclude::[]\n",
			want: `
Opaque "image::tiger.png[Tiger]\n"
Opaque "toc::[]\n"
Opaque "foo::bar[]\n"
Opaque "include::[]\n"`,
		},
		{
			name: "a block macro target may hold a bracket",
			src:  "image::[x][]\n",
			want: `
Opaque "image::[x][]\n"`,
		},
		{
			name: "a style only an extension knows keeps the block as it is",
			src:  "[mermaid]\ngraph TD\nA-->B\n\n[mermaid]\n--\ngraph TD\n--\n",
			want: `
Opaque meta("[mermaid]\n") "graph TD\nA-->B\n"
Verbatim meta("[mermaid]\n") "--\ngraph TD\n--\n"`,
		},
		{
			name: "a style counts only in its case, as in Asciidoctor",
			src:  "[Quote]\n\"A quote.\"\n-- Author\n\n[NOTE]\nText.\n\n[note]\nText.\n",
			want: `
Opaque meta("[Quote]\n") "\"A quote.\"\n-- Author\n"
Paragraph meta("[NOTE]\n") "Text.\n"
Opaque meta("[note]\n") "Text.\n"`,
		},
		{
			name: "a label, a colon and whitespace open an admonition paragraph",
			src:  "NOTE: Text.\nMore.\n\nTIP:\tText.\n\nNOTE:\nText.\n\nNote: Text.\n",
			want: `
Admonition "NOTE: Text.\nMore.\n"
Admonition "TIP:\tText.\n"
Paragraph "NOTE:\nText.\n"
Paragraph "Note: Text.\n"`,
		},
		{
			name: "a paragraph an item carries can be an admonition, the item text cannot",
			src:  "* NOTE: Text.\n+\nNOTE: Text.\n",
			want: `
List "*"
  ListItem "* NOTE: Text.\n"
    Continuation gap! "+\n"
    Admonition "NOTE: Text.\n"`,
		},
		{
			name: "substitutions of its own make a paragraph literal",
			src:  "[subs=none]\n<pre>One. Two.</pre>\n\n[.lead,subs=\"-quotes\"]\nText.\n\n[.subs]\nText.\n",
			want: `
Literal meta("[subs=none]\n") "<pre>One. Two.</pre>\n"
Literal meta("[.lead,subs=\"-quotes\"]\n") "Text.\n"
Paragraph meta("[.subs]\n") "Text.\n"`,
		},
		{
			name: "a Markdown quote is a quote block, not a paragraph",
			src:  "> A famous quote.\n> -- Famous Person\n",
			want: `
Opaque "> A famous quote.\n> -- Famous Person\n"`,
		},
		{
			name: "so is a quoted paragraph with an attribution as its last line",
			src:  "\"A famous quote.\nSome more inspiring words.\"\n-- Famous Person, Famous Source\n",
			want: `
Opaque "\"A famous quote.\nSome more inspiring words.\"\n-- Famous Person, Famous Source\n"`,
		},
		{
			name: "without the attribution as its last line the quotes are prose",
			src:  "\"A quote.\"\nMore text.\n\n\"A quote.\"\n-- Author\nMore.\n\n\"A quote.\"\nmore\n-- Author\n",
			want: `
Paragraph "\"A quote.\"\nMore text.\n"
Paragraph "\"A quote.\"\n-- Author\nMore.\n"
Paragraph "\"A quote.\"\nmore\n-- Author\n"`,
		},
		{
			name: "only straight quotes and exactly two dashes and a space count, and a quote style reads the attribution as text",
			src:  "“A quote.”\n-- Author\n\n\"A quote.\"\n--Author\n\n[quote]\n\"A quote.\"\n-- Author\n\n[.lead]\n\"A quote.\"\n// a comment\n-- Author\n",
			want: `
Paragraph "“A quote.”\n-- Author\n"
Paragraph "\"A quote.\"\n--Author\n"
Paragraph meta("[quote]\n") "\"A quote.\"\n-- Author\n"
Opaque! meta("[.lead]\n") "\"A quote.\"\n// a comment\n-- Author\n"`,
		},
		{
			name: "a Markdown quote that opens with a term is a list",
			src:  "> term:: Text.\n",
			want: `
List "::"
  ListItem "> term:: Text.\n"`,
		},
		{
			name: "a term with no text of its own takes the lines below it",
			src:  "term1::\n\n'''\ncontinued\n",
			want: `
List "::"
  ListItem "term1::\n\n'''\ncontinued\n"`,
		},
		{
			name: "a term with a line right under it takes nothing below a blank line",
			src:  "term1::\ntext\n\n=== B\n\nterm2::\n// a comment\n\nmore\n",
			want: `
List "::"
  ListItem "term1::\ntext\n"
Heading "=== B\n" title "B"
List "::"
  ListItem "term2::\n// a comment\n"
Paragraph "more\n"`,
		},
		{
			name: "a + with no list around it opens a paragraph of its own",
			src:  "text\n+\n- a\n+\n1. a\n",
			want: `
Paragraph "text\n"
Paragraph! gap! "+\n- a\n"
Paragraph! gap! "+\n1. a\n"`,
		},
		{
			name: "a + right under the one carrying it is text again",
			src:  "* one\n+\n+\n1. two\n\n1. three\n",
			want: `
List "*"
  ListItem "* one\n"
    Continuation gap! "+\n"
    Paragraph! gap! "+\n1. two\n"
    List "1."
      ListItem "1. three\n"`,
		},
		{
			name: "a blank line between the two keeps the second one carrying",
			src:  "* one\n+\n\n+\ntext\n",
			want: `
List "*"
  ListItem "* one\n"
    Continuation gap! "+\n"
    Continuation gap! "+\n"
    Paragraph "text\n"`,
		},
		{
			name: "metadata above a + binds to the block below it, which a verbatim style makes code",
			src:  "* a\n[literal]\n+\nOne. Two.\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal gap! meta("[literal]\n") "One. Two.\n"`,
		},
		{
			name: "it reads past blank lines above the + too",
			src:  "* a\n[literal]\n\n+\nOne. Two.\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal gap! meta("[literal]\n") "One. Two.\n"`,
		},
		{
			name: "a + right under the one it reads past is a line of the block, which stays as it stands",
			src:  "* a\n[literal]\n+\n+\nOne. Two.\n",
			want: `
List "*"
  ListItem "* a\n"
    Paragraph! gap! meta("[literal]\n") "+\nOne. Two.\n"`,
		},
		{
			name: "past a nested list the item holds the + as it stands, so it is text under the metadata",
			src:  "* a\nb::\n[literal]\n+\nOne. Two.\n",
			want: `
List "*"
  ListItem "* a\n"
    List "::"
      ListItem "b::\n"
    Paragraph! gap! meta("[literal]\n") "+\nOne. Two.\n"`,
		},
		{
			name: "it binds the same way in a nested item and under a +",
			src:  "* a\n** b\n[literal]\n+\nOne. Two.\n\ntext\n\nt:: d\n+\n[verse]\n+\nOne. Two.\n",
			want: `
List "*"
  ListItem "* a\n"
    List "**"
      ListItem "** b\n"
        Literal gap! meta("[literal]\n") "One. Two.\n"
Paragraph "text\n"
List "::"
  ListItem "t:: d\n"
    Continuation gap! "+\n"
    Literal gap! meta("[verse]\n") "One. Two.\n"`,
		},
		{
			name: "right under the item's line a block title is text, which a verbatim style makes code",
			src:  "* b\n[source]\n+\n.T\n\ntext\n",
			want: `
List "*"
  ListItem "* b\n"
    Literal gap! meta("[source]\n") ".T\n"
    Paragraph "text\n"`,
		},
		{
			name: "comment lines between the item's line and the metadata change nothing",
			src:  "* b\n// c\n[source]\n+\n.T\n\ntext\n",
			want: `
List "*"
  ListItem "* b\n// c\n"
    Literal gap! meta("[source]\n") ".T\n"
    Paragraph "text\n"`,
		},
		{
			name: "a directive there counts as metadata, so the block title under it is text",
			src:  "* a\n[.r]\nifdef::x[]\n.T\nendif::[]\n",
			want: `
List extensible! "*"
  ListItem "* a\n"
    Paragraph! gap! meta("[.r]\n") meta("ifdef::x[]\n") ".T\nendif::[]\n"`,
		},
		{
			name: "so is an attribute entry",
			src:  ". o\n[literal]\n:a: b\ntext\n",
			want: `
List "."
  ListItem ". o\n"
    Literal meta("[literal]\n") ":a: b\ntext\n"`,
		},
		{
			name: "and so is a break, which a comment style reads on past an item line",
			src:  "* a\n[comment]\n---\nc:: d\n",
			want: `
List "*"
  ListItem "* a\n"
    Literal! meta("[comment]\n") "---\nc:: d\n"`,
		},
		{
			name: "and so is a block macro, so a + under the paragraph carries the block below",
			src:  "* b\n[%hardbreaks]\nimage::a.png[]\n.T\n+\ntext\nc:: d\n",
			want: `
List "*"
  ListItem "* b\n"
    Opaque meta("[%hardbreaks]\n") "image::a.png[]\n.T\n"
    Continuation gap! "+\n"
    Paragraph! "text\nc:: d\n"`,
		},
		{
			name: "under more text of the item a block title is metadata again",
			src:  "* b\nmore\n[source]\n+\n.T\n\ntext\n",
			want: `
List "*"
  ListItem "* b\nmore\n"
    Literal meta("[source]\n") meta(".T\n") "text\n"`,
		},
		{
			name: "under a + a block title is metadata again",
			src:  "* b\n+\n[source]\n+\n.T\n\ntext\n",
			want: `
List "*"
  ListItem "* b\n"
    Continuation gap! "+\n"
    Literal meta("[source]\n") meta(".T\n") "text\n"`,
		},
		{
			name: "the paragraph below the + still ends at an item line",
			src:  "* a\n[.r]\n+\ntext\n** n\n",
			want: `
List "*"
  ListItem "* a\n"
    Paragraph gap! meta("[.r]\n") "text\n"
    List "**"
      ListItem "** n\n"`,
		},
		{
			name: "unless a paragraph style hands it to block parsing",
			src:  "* a\n[quote]\n+\ntext\nc:: d\n",
			want: `
List "*"
  ListItem "* a\n"
    Paragraph! gap! meta("[quote]\n") "text\nc:: d\n"`,
		},
		{
			name: "which also reads a quote-shaped paragraph past an item line",
			src:  "* a\n[sidebar]\n> x\nc:: d\n",
			want: `
List "*"
  ListItem "* a\n"
    Opaque! meta("[sidebar]\n") "> x\nc:: d\n"`,
		},
		{
			name: "where no block follows the +, the metadata binds to nothing",
			src:  "* a\nb:: c\n+\n.T\n\n\n+\n\nc:: d\n",
			want: `
List "*"
  ListItem "* a\n"
    List "::"
      ListItem "b:: c\n"
        Continuation gap! "+\n"
        Opaque gap! meta(".T\n") ""
      ListItem "c:: d\n"`,
		},
		{
			name: "a block attribute line under a term ends the description list",
			src:  "term::\n[verse]\ndetached\n",
			want: `
List "::"
  ListItem "term::\n"
Literal meta("[verse]\n") "detached\n"`,
		},
		{
			name: "it stays in the item where a list follows it instead",
			src:  "term::\n[start=3]\n1. one\n",
			want: `
List "::"
  ListItem "term::\n"
    List meta("[start=3]\n") "1."
      ListItem "1. one\n"`,
		},
		{
			name: "it ends the list above the next term too",
			src:  "a:: x\n[source]\nb:: y\nc:: z\n",
			want: `
List "::"
  ListItem "a:: x\n"
Literal! meta("[source]\n") "b:: y\nc:: z\n"`,
		},
		{
			name: "it ends the description list from inside a list nested in it",
			src:  "a:: x\n* y\n[.r]\nmore\n",
			want: `
List "::"
  ListItem "a:: x\n"
    List "*"
      ListItem "* y\n"
Paragraph meta("[.r]\n") "more\n"`,
		},
		{
			name: "it stays where an item of the nested list follows it",
			src:  "a:: x\n* y\n[.r]\n* z\n",
			want: `
List "::"
  ListItem "a:: x\n"
    List "*"
      ListItem "* y\n"
        Opaque meta("[.r]\n") ""
      ListItem "* z\n"`,
		},
		{
			name: "a directive among the lines it takes freezes the gap after them",
			src:  "term1::\ninclude::part.adoc[]\n\ntext\n\nterm2:: d\n",
			want: `
List extensible! "::"
  ListItem! "term1::\ninclude::part.adoc[]\n\ntext\n"
  ListItem gap! "term2:: d\n"`,
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
			src:  "----  \ncode\n----\f\n\nAfter.\n",
			want: `
Verbatim "----  \ncode\n----\f\n"
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
Header gap! "= Title\n" title "Title"`,
		},
		{
			name: "front matter freezes the blocks and gaps down to the next blank line",
			src:  "---\ntitle: x\n---\n:a: 1\n== Sec\n\nText.\n",
			want: `
FrontMatter "---\ntitle: x\n---\n"
Attribute! gap! ":a: 1\n"
Heading! gap! "== Sec\n" title "Sec"
Paragraph gap! "Text.\n"`,
		},
		{
			name: "a blank line above a fence pair is what keeps it from being front matter",
			src:  "\n---\ntitle: x\n---\n= Title\n",
			want: `
Opaque gap! "---\n"
Paragraph "title: x\n---\n= Title\n"`,
		},
		{
			name: "a blank line above a lone fence stays as well",
			src:  "\n---\ntitle: x\n\n= Title\n",
			want: `
Opaque gap! "---\n"
Paragraph "title: x\n"
Heading "= Title\n" title "Title"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			doc, err := Parse([]byte(test.src))
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

func TestParseReports(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Finding
	}{
		{
			name: "a block that never closes",
			src:  "----\ncode\n",
			want: []Finding{{Line: 1, Message: "block has no closing delimiter"}},
		},
		{
			name: "a comment block among the metadata lines that never closes",
			src:  "[source]\n////\nc\n",
			want: []Finding{{Line: 2, Message: "block has no closing delimiter"}},
		},
		{
			name: "a comment block in the header that never closes",
			src:  "= Title\n:a: b\n////\nc\n\n* d\n",
			want: []Finding{{Line: 3, Message: "block has no closing delimiter"}},
		},
		{
			name: "a block under a literal paragraph in an item that does not close before the item ends",
			src:  "* a\n\n  code\n----\nx\n\ny\n----\n",
			want: []Finding{
				{Line: 4, Message: "block has no closing delimiter"},
				{Line: 8, Message: "block has no closing delimiter"},
			},
		},
		{
			name: "a fence under a line of text there opens a block rather than underlining the text",
			src:  "* a\n\n  code\n....\ny\n....\ntext\n----\n",
			want: []Finding{{Line: 8, Message: "block has no closing delimiter"}},
		},
		{
			name: "a fence inside a verbatim paragraph under such a literal paragraph opens no block",
			src:  "* a\n\n  code\n[source]\nx\n----\n\nText.\n",
			want: nil,
		},
		{
			name: "a block under an item a directive reaches is read as if the directive brought in nothing",
			src:  "include::part.adoc[]\n* b\n[source]\nc\n----\nx\n",
			want: []Finding{{Line: 5, Message: "block has no closing delimiter"}},
		},
		{
			name: "a fence two blank lines under a term without text, which opens a block in the item",
			src:  "t::\n\n\n----\nx\n----\n",
			want: []Finding{
				{Line: 4, Message: "block has no closing delimiter"},
				{Line: 6, Message: "block has no closing delimiter"},
			},
		},
		{
			name: "a fence two blank lines under a term a directive reaches is refused, since the directive reads as bringing in nothing",
			src:  "t::\ninclude::part.adoc[]\n\n\n----\nx\n----\n",
			want: []Finding{
				{Line: 5, Message: "block has no closing delimiter"},
				{Line: 7, Message: "block has no closing delimiter"},
			},
		},
		{
			name: "a delimiter that closes outside the region it opened in",
			src:  "ifdef::extra[]\n----\nendif::[]\ncode\n----\n",
			want: []Finding{{Line: 2, Message: "delimiter opens and closes in different conditional regions"}},
		},
		{
			name: "a line of the body that closes the block Asciidoctor ended there",
			src:  "====\n----\n====\n----\n====\n",
			want: []Finding{{Line: 3, Message: "closes the block opened above it"}},
		},
		{
			name: "also where the line opens a block of its own further in",
			src:  "======\n====\n======\ntext\n======\n====\n======\n",
			want: []Finding{{Line: 3, Message: "closes the block opened above it"}},
		},
		{
			name: "a conditional inside a [comment] on a wider fence opens a region all the same",
			src:  "[comment]\n----\nifdef::extra[]\n----\nText.\n",
			want: []Finding{
				{Line: 2, Message: "delimiter opens and closes in different conditional regions"},
				{Line: 3, Message: "conditional region has no endif"},
			},
		},
		{
			name: "also when the conditional sits among the metadata lines",
			src:  "[source]\nifdef::extra[]\n----\nendif::[]\ncode\n----\n",
			want: []Finding{{Line: 3, Message: "delimiter opens and closes in different conditional regions"}},
		},
		{
			name: "a block that never closes reports both when it also left its region",
			src:  "ifdef::extra[]\n----\nendif::[]\ncode\n",
			want: []Finding{
				{Line: 2, Message: "block has no closing delimiter"},
				{Line: 2, Message: "delimiter opens and closes in different conditional regions"},
			},
		},
		{
			name: "a conditional that never closes, and an endif that closes nothing",
			src:  "endif::[]\n\nifdef::extra[]\nText.\n",
			want: []Finding{
				{Line: 1, Message: "endif closes no conditional region"},
				{Line: 3, Message: "conditional region has no endif"},
			},
		},
		{
			name: "an open region is decided last and reported first all the same",
			src:  "ifdef::extra[]\n\n----\ncode\n",
			want: []Finding{
				{Line: 1, Message: "conditional region has no endif"},
				{Line: 3, Message: "block has no closing delimiter"},
			},
		},
		{
			name: "a directive is not a finding, the parser freezes around it instead",
			src:  "include::part.adoc[]\n\nifdef::extra[]\nText.\nendif::[]\n",
			want: nil,
		},
		{
			name: "a conditional with text in the brackets opens no region",
			src:  "Text.\nifdef::extra[Only then.]\nMore.\n",
			want: nil,
		},
		{
			name: "a malformed directive is a finding, as it is an error in Asciidoctor",
			src:  "ifdef::[]\nifeval::target[1 == 1]\nifeval::[]\nifeval::[1 | 2]\nendif::a[x]\nendif::[]\n",
			want: []Finding{
				{Line: 1, Message: "malformed preprocessor directive"},
				{Line: 2, Message: "malformed preprocessor directive"},
				{Line: 3, Message: "malformed preprocessor directive"},
				{Line: 4, Message: "malformed preprocessor directive"},
				{Line: 5, Message: "malformed preprocessor directive"},
				{Line: 6, Message: "endif closes no conditional region"},
			},
		},
		{
			name: "an ifeval expression is stripped the way Ruby strips it",
			src:  "ifeval::[a <\u00a0]\nendif::[]\nifeval::[a <\x00]\nendif::[]\n",
			want: []Finding{
				{Line: 3, Message: "malformed preprocessor directive"},
				{Line: 4, Message: "endif closes no conditional region"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			doc, err := Parse([]byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(doc.Findings, test.want) {
				t.Errorf("got %+v, want %+v", doc.Findings, test.want)
			}
		})
	}
}

// TestParseRejects covers what the parser must not repair silently.
func TestParseRejects(t *testing.T) {
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

			if _, err := Parse([]byte(test.src)); err == nil {
				t.Error("got no error, want one")
			}
		})
	}
}

// TestParseLineEnding covers the answer a rule writing a line of its own reads.
func TestParseLineEnding(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want LineEnding
	}{
		{name: "line feeds", src: "a\nb\n", want: LF},
		{name: "carriage return line feeds", src: "a\r\nb\r\n", want: CRLF},
		{name: "no line ending at all", src: "a", want: LF},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			doc, err := Parse([]byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			if doc.LineEnding != test.want {
				t.Errorf("got %q, want %q", doc.LineEnding, test.want)
			}
		})
	}
}

// TestReadsAs pins what a rewrite of a node's lines must not turn them into.
func TestReadsAs(t *testing.T) {
	tests := []struct {
		name  string
		node  Node
		lines string
		want  bool
	}{
		{"prose stays prose", &Paragraph{}, "One.\nTwo.\n", true},
		{"a label stays an admonition", &Admonition{}, "NOTE: One.\nTwo.\n", true},
		{"a join makes a label", &Paragraph{}, "NOTE: One.\n", false},
		{"a split starts a list", &Paragraph{}, "* One.\n", false},
		{"a later line starts an item", &Paragraph{}, "One.\n. Two.\n", false},
		{"a split makes an attribute line", &Paragraph{}, "[Optional.]\nSet it.\n", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := ReadsAs(test.node, []byte(test.lines)); got != test.want {
				t.Errorf("ReadsAs(%T, %q) = %t, want %t", test.node, test.lines, got, test.want)
			}
		})
	}
}
