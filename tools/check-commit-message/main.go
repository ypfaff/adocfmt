// Command check-commit-message reads a commit message from stdin and checks
// that it follows Conventional Commits the way this repository writes them:
//
//	git log -1 --format=%B | go run ./tools/check-commit-message
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GitHub cuts longer subjects short.
const maxSubjectLength = 72

var types = []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}

// type(scope)!: description, where the scope and the ! are optional.
var subjectForm = regexp.MustCompile(`^([a-z]+)(?:\([a-z0-9-]+\))?!?: (\S.*)$`)

// Git marks the end of the message this way in git commit --verbose and puts
// the diff below it.
const scissors = "# ------------------------ >8 ------------------------"

func main() {
	log.SetFlags(0)

	message, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalln("error:", err)
	}
	problems := check(string(message))
	for _, problem := range problems {
		log.Println("error:", problem)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

func check(message string) []string {
	lines := messageLines(message)
	if len(lines) == 0 {
		return []string{"the message is empty"}
	}
	problems := checkSubject(lines[0])
	if len(lines) > 1 && lines[1] != "" {
		problems = append(problems, "a blank line must separate the subject from the body")
	}
	return problems
}

func checkSubject(subject string) []string {
	var problems []string
	if length := utf8.RuneCountInString(subject); length > maxSubjectLength {
		problems = append(problems, fmt.Sprintf("the subject has %d characters, more than %d", length, maxSubjectLength))
	}

	match := subjectForm.FindStringSubmatch(subject)
	if match == nil {
		return append(problems, fmt.Sprintf("the subject %q is not of the form type(scope)!: description, in lowercase", subject))
	}
	kind, description := match[1], match[2]
	if !slices.Contains(types, kind) {
		problems = append(problems, fmt.Sprintf("the type %q is none of %s", kind, strings.Join(types, ", ")))
	}
	if first, _ := utf8.DecodeRuneInString(description); unicode.IsUpper(first) {
		problems = append(problems, "the description starts with an uppercase letter")
	}
	return problems
}

// The commit-msg hook gets the message before Git drops its comments and
// trailing whitespace.
func messageLines(message string) []string {
	var lines []string
	for line := range strings.Lines(message) {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line == scissors {
			break
		}
		if !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}
