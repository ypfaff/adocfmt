// Package format turns AsciiDoc source into its formatted form.
package format

import (
	"errors"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/printer"
)

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
		errs := make([]error, len(doc.Findings))
		for i, finding := range doc.Findings {
			errs[i] = finding
		}
		return nil, errors.Join(errs...)
	}
	return printer.Print(doc), nil
}
