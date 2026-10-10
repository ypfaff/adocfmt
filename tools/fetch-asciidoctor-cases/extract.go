package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// heredocStart matches the opening of a test input.
	//
	// Only single-quoted heredocs are taken. The quotes stop Ruby from
	// interpolating, so the body is literal AsciiDoc; the unquoted form may hold
	// #{...} expressions, which would reach the cases as broken markup.
	heredocStart = regexp.MustCompile(`input = <<~'([A-Za-z_]+)'`)

	// testName matches the enclosing `test '...' do`, whose name becomes the
	// file name of every case below it.
	//
	// The body is greedy on purpose: it spans the first quote to the last one on
	// the line, which keeps an apostrophe inside a double-quoted name from
	// cutting the name short.
	testName = regexp.MustCompile(`^\s*test\s+['"](.*)['"]`)

	unsafeInName = regexp.MustCompile(`[^a-z0-9]+`)
)

// testCase is one AsciiDoc input lifted out of a Ruby test file.
type testCase struct {
	name string
	body string
}

// extract returns every test input of one Ruby test file, in source order.
func extract(ruby string) []testCase {
	var (
		cases      []testCase
		name       string
		terminator string
		body       []string
		capturing  bool
	)

	for _, line := range strings.Split(ruby, "\n") {
		if capturing {
			if strings.TrimSpace(line) == terminator {
				capturing = false
				if dedented, ok := dedent(body); ok {
					cases = append(cases, testCase{name: name, body: dedented})
				}
			} else {
				body = append(body, line)
			}
			continue
		}

		if match := testName.FindStringSubmatch(line); match != nil {
			name = fileName(match[1])
		}
		if match := heredocStart.FindStringSubmatch(line); match != nil {
			terminator, capturing, body = match[1], true, nil
		}
	}
	return cases
}

// dedent applies the rule of Ruby's squiggly heredoc: strip the indentation of
// the least-indented non-blank line. It reports false for a body that holds
// nothing but blank lines.
func dedent(lines []string) (string, bool) {
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if n := len(line) - len(strings.TrimLeft(line, " ")); indent < 0 || n < indent {
			indent = n
		}
	}
	if indent < 0 {
		return "", false
	}

	var out strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out.WriteString(line[indent:])
		}
		out.WriteString("\n")
	}
	return out.String(), true
}

// fileName turns a Ruby test name into a file name that survives every file
// system and stays readable in test output.
func fileName(testName string) string {
	name := unsafeInName.ReplaceAllString(strings.ToLower(testName), "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "case"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

// path is where a case goes, relative to the cases directory.
func (c testCase) path(dir string, number int) string {
	return fmt.Sprintf("%s/%04d-%s.adoc", dir, number, c.name)
}
