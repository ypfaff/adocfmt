package main

import (
	"reflect"
	"testing"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name string
		ruby string
		want []testCase
	}{
		{
			name: "strips the indentation of the least-indented line",
			ruby: "  test 'a sidebar' do\n    input = <<~'EOS'\n    == Section\n\n    ****\n    body\n    ****\n    EOS\n  end\n",
			want: []testCase{{name: "a-sidebar", body: "== Section\n\n****\nbody\n****\n"}},
		},
		{
			// Indentation relative to that line carries meaning in AsciiDoc, so
			// only the common part may go.
			name: "keeps relative indentation",
			ruby: "  test 'hanging indent' do\n    input = <<~'EOS'\n    - item\n      continued\n    EOS\n  end\n",
			want: []testCase{{name: "hanging-indent", body: "- item\n  continued\n"}},
		},
		{
			name: "ignores an interpolating heredoc",
			ruby: "  test 'interpolated' do\n    input = <<~EOS\n    #{value}\n    EOS\n  end\n",
			want: nil,
		},
		{
			name: "skips a body of blank lines",
			ruby: "  test 'blank' do\n    input = <<~'EOS'\n\n    EOS\n  end\n",
			want: nil,
		},
		{
			// The name spans to the last quote on the line, so the apostrophe
			// does not cut it short.
			name: "keeps an apostrophe inside a double-quoted name",
			ruby: "  test \"honors the author's name\" do\n    input = <<~'EOS'\n    text\n    EOS\n  end\n",
			want: []testCase{{name: "honors-the-author-s-name", body: "text\n"}},
		},
		{
			name: "takes every case of a file in order",
			ruby: "  test 'first' do\n    input = <<~'EOS'\n    one\n    EOS\n  end\n  test 'second' do\n    input = <<~'EOS'\n    two\n    EOS\n  end\n",
			want: []testCase{{name: "first", body: "one\n"}, {name: "second", body: "two\n"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := extract(test.ruby); !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestFileName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Should Parse A Block", "should-parse-a-block"},
		{"a -- b", "a-b"},
		{"...", "case"},
		{"an extremely long test name that runs past the limit the file name allows", "an-extremely-long-test-name-that-runs-past-the-limit-the-fil"},
	}
	for _, test := range tests {
		if got := fileName(test.in); got != test.want {
			t.Errorf("fileName(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
