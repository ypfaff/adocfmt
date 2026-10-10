// Package format turns AsciiDoc source into its formatted form.
package format

import (
	"strings"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/printer"
)

// Finding is one thing the parser could not decide, and the line it stands on.
type Finding = block.Finding

// Refusal is what Format returns for a document the parser reported findings
// on, in line order.
//
// The findings stay apart rather than joined into one message, so a caller can
// write the path it read in front of each of them.
type Refusal struct{ Findings []Finding }

// Error joins the findings, one per line.
//
// Refusal is exported, so one can reach here empty or nil. Neither is a
// document the parser turned down, and saying so beats an empty message or a
// panic.
func (r *Refusal) Error() string {
	if r == nil || len(r.Findings) == 0 {
		return "refused without a finding"
	}
	lines := make([]string, len(r.Findings))
	for at, finding := range r.Findings {
		lines[at] = finding.Error()
	}
	return strings.Join(lines, "\n")
}

// Format returns the formatted form of src.
//
// A document the parser reported findings on is refused with all of them,
// because formatting the part it did understand would report success on a
// document it barely touched.
//
// A document that says adocfmt: ignore-file above its title comes back as it
// is, unread, so a refusal behind the directive goes unreported too.
func Format(src []byte) ([]byte, error) {
	if block.IgnoresFile(src) {
		return src, nil
	}
	doc, err := block.Parse(src)
	if err != nil {
		return nil, err
	}
	if len(doc.Findings) > 0 {
		return nil, &Refusal{Findings: doc.Findings}
	}
	return printer.Print(doc), nil
}
