package sentence

import (
	"bytes"
	"testing"
)

// TestReflow pins where a sentence ends and what the reflow keeps. The input
// and the result are lines joined with a line end, so a case reads like the
// paragraph it stands for.
func TestReflow(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"one sentence", "One sentence.", "One sentence."},
		{"joined", "First sentence\ncontinues here.", "First sentence continues here."},
		{"split", "One. Two. Three.", "One.\nTwo.\nThree."},
		{"joined and split", "First sentence\ncontinues here. Second one.", "First sentence continues here.\nSecond one."},
		{"already split", "One.\nTwo.", "One.\nTwo."},
		{"indented line", "One\n  two.", "One two."},
		{"several spaces", "One.   Two.", "One.\nTwo."},
		{"tab", "One.\tTwo.", "One.\nTwo."},
		{"spaces inside a line", "One  two.", "One  two."},

		{"exclamation", "Stop! Go.", "Stop!\nGo."},
		{"question", "Why? Because.", "Why?\nBecause."},
		{"lowercase after exclamation", "Wow! nice.", "Wow! nice."},
		{"lowercase after question", "Why? because.", "Why? because."},
		{"digit after exclamation", "Wow! 3 more.", "Wow! 3 more."},
		{"lowercase after period", "See the file. next", "See the file. next"},
		{"digit after period", "Step one. 2 more.", "Step one. 2 more."},
		{"umlaut", "Eins. Ärger.", "Eins.\nÄrger."},
		{"no whitespace after the mark", "Done.footnote:[One. Two.] Next.", "Done.footnote:[One. Two.] Next."},

		{"closing quote", `He said "Stop." Then left.`, "He said \"Stop.\"\nThen left."},
		{"closing curly quote", "Er sagte „Halt.“ Dann ging er.", "Er sagte „Halt.“\nDann ging er."},
		{"closing bracket", "It works (mostly.) Next.", "It works (mostly.)\nNext."},
		{"closing markup", "This is *important.* Next.", "This is *important.*\nNext."},
		{"opening quote", `One. "Two."`, "One.\n\"Two.\""},
		{"opening markup", "One. *Two.*", "One.\n*Two.*"},
		{"opening bracket", "One. (Two.)", "One.\n(Two.)"},
		{"opening german quote", "Eins. „Zwei.“", "Eins.\n„Zwei.“"},

		{"ordinal", "Am 3. Oktober.", "Am 3. Oktober."},
		{"initial", "By A. Smith.", "By A. Smith."},
		{"lowercase initial", "Siehe S. 3 und z. B. Kapitel 2.", "Siehe S. 3 und z. B. Kapitel 2."},
		{"latin abbreviation", "Some, e.g. Linux.", "Some, e.g. Linux."},
		{"english abbreviation", "Ask Dr. Who.", "Ask Dr. Who."},
		{"german abbreviation", "Siehe Nr. 5 bzw. Abb. Sieben.", "Siehe Nr. 5 bzw. Abb. Sieben."},
		{"abbreviation any case", "Apples, pears ETC. More.", "Apples, pears ETC. More."},
		{"ellipsis", "Wait... Then.", "Wait... Then."},
		{"unicode ellipsis", "Wait… Then.", "Wait… Then."},
		{"period after a bracket", "It works (see x). Next.", "It works (see x).\nNext."},

		{"lowercase after a period at the line end", "One ends.\nadocfmt starts.", "One ends.\nadocfmt starts."},
		{"macro after a period at the line end", "One ends.\nlink:x[Two] starts.", "One ends.\nlink:x[Two] starts."},
		{"code after a question at the line end", "Why?\n`go` answers.", "Why?\n`go` answers."},
		{"colon at the line end", "Three modes:\ncheck and write.", "Three modes:\ncheck and write."},
		{"colon inside a line", "Note: This. Next.", "Note: This.\nNext."},
		{"number at the line end", "It exits with 0.\nThe build fails.", "It exits with 0.\nThe build fails."},
		{"abbreviation at the line end", "Apples, pears etc.\nthen more.", "Apples, pears etc.\nthen more."},
		{"initial at the line end", "By A.\nSmith.", "By A.\nSmith."},
		{"closing quote at the line end", "He said \"Stop.\"\nthen left.", "He said \"Stop.\"\nthen left."},
		{"ellipsis at the line end", "Wait...\nthen it came.", "Wait...\nthen it came."},
		{"unicode ellipsis at the line end", "Wait…\nthen it came.", "Wait… then it came."},
		{"period in a backtick span at the line end", "Run `make.`\nnow.", "Run `make.` now."},

		{"hard break", "One. +\nTwo. Three.", "One. +\nTwo.\nThree."},
		{"hard break keeps indentation", "One +\n  two.", "One +\n  two."},
		{"two hard breaks", "One +\nTwo. Three +\nFour. Five.", "One +\nTwo.\nThree +\nFour.\nFive."},
		{"plus between words", "One + Two. Three.", "One + Two.\nThree."},
		{"plus alone on a line indented by a tab", "One\n\t+\ntwo.", "One + two."},
		{"plus after a tab", "One\t+\ntwo.", "One\t+ two."},
		{"plus alone on a line indented by spaces", "One\n  +\ntwo.", "One +\ntwo."},

		{"backtick span", "Run `a. B` now. Next.", "Run `a. B` now.\nNext."},
		{"double backtick span", "Run ``a. B`` now.", "Run ``a. B`` now."},
		{"period in a backtick span", "Run `make.` Next.", "Run `make.` Next."},
		{"passthrough", "Use +a. B+ here.", "Use +a. B+ here."},
		{"plus inside words", "Use x+y here. Then a+b works.", "Use x+y here.\nThen a+b works."},
		{"backtick inside a word", "It`s here. Next `x` one.", "It`s here.\nNext `x` one."},
		{"double passthrough", "Use ++a. B++ here.", "Use ++a. B++ here."},
		{"triple passthrough", "Use +++<b>a. B</b>+++ here.", "Use +++<b>a. B</b>+++ here."},
		{"line end in a backtick span", "Run `a\nb` now.", "Run `a\nb` now."},
		{"line end in a passthrough", "Use +++<pre>a\nb</pre>+++ here.", "Use +++<pre>a\nb</pre>+++ here."},
		{"pass macro", "Use pass:[a. B] here.", "Use pass:[a. B] here."},
		{"link macro", "See link:x.html[Text. More] now.", "See link:x.html[Text. More] now."},
		{"url macro", "See https://x.org[Text. More] now.", "See https://x.org[Text. More] now."},
		{"nested brackets", "See image:a.png[Alt [x]. More] now.", "See image:a.png[Alt [x]. More] now."},
		{"line end in a macro", "See link:x.html[Text\nmore] now.", "See link:x.html[Text\nmore] now."},
		{"space in an image target", "The image:Step one. Setup.png[] shows it.", "The image:Step one. Setup.png[] shows it."},
		{"line end in an image target", "See image:my\nfile.png[] now.", "See image:my\nfile.png[] now."},
		{"space in an xref target", "Read xref:Step one. Setup.adoc[] now.", "Read xref:Step one. Setup.adoc[] now."},
		{"space at the end of an xref target", "Read xref:Step one. Setup []. Next.", "Read xref:Step one. Setup [].\nNext."},
		{"macro name in prose", "See the image: it helps. Next[1] too.", "See the image: it helps.\nNext[1] too."},
		{"cross reference", "See <<id,Text. More>> now.", "See <<id,Text. More>> now."},
		{"anchor", "Here [[id,Text. More]] now.", "Here [[id,Text. More]] now."},
		{"index term", "Here ((Term. More)) now.", "Here ((Term. More)) now."},
		{"counter", "See {counter:one. Two} now.", "See {counter:one. Two} now."},
		{"line end in a counter", "See {counter2:a\nb} now.", "See {counter2:a\nb} now."},
		{"brace right after a counter colon", "See {counter:}\n} now.", "See {counter:}\n} now."},
		{"line end in other braces", "See {a\nb} now. Next.", "See {a b} now.\nNext."},
		{"escaped backtick", "A \\` bb. Cc.", "A \\` bb.\nCc."},
		{"unclosed backtick", "A ` bb. Cc.", "A ` bb.\nCc."},
		{"unclosed bracket", "See x[aa. Bb", "See x[aa.\nBb"},

		{"empty", "", ""},
		{"lone mark", ".", "."},
		{"ending in an opener", "One. (", "One. ("},
		{"ending in a counter", "One. {counter:", "One. {counter:"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reflow(test.src); got != test.want {
				t.Errorf("Reflow(%q) = %q, want %q", test.src, got, test.want)
			}
			// Formatting a formatted document changes nothing.
			if got := reflow(test.want); got != test.want {
				t.Errorf("Reflow(%q) = %q, want it unchanged", test.want, got)
			}
		})
	}
}

func reflow(src string) string {
	return string(bytes.Join(Reflow(bytes.Split([]byte(src), []byte("\n"))), []byte("\n")))
}
