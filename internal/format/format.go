// Package format turns AsciiDoc source into its formatted form.
package format

import (
	"strings"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/printer"
)

// Finding is one thing the scanner could not decide, and the line it stands on.
type Finding = block.Finding

// Refusal is what Format returns for a document the scanner reported findings
// on, in line order.
//
// The findings stay apart rather than joined into one message, so a caller can
// write the path it read in front of each of them.
type Refusal struct{ Findings []Finding }

// Error implements error.
func (r *Refusal) Error() string {
	lines := make([]string, len(r.Findings))
	for at, finding := range r.Findings {
		lines[at] = finding.Error()
	}
	return strings.Join(lines, "\n")
}

// Format returns the formatted form of src.
//
// A document the scanner reported findings on is refused with all of them,
// because formatting the part it did understand would report success on a
// document it barely touched.
func Format(src []byte) ([]byte, error) {
	doc, err := block.Scan(src)
	if err != nil {
		return nil, err
	}
	if len(doc.Findings) > 0 {
		return nil, &Refusal{Findings: doc.Findings}
	}
	return printer.Print(doc), nil
}
