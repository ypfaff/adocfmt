// Package format turns AsciiDoc source into its formatted form.
package format

import (
	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/printer"
)

// Format returns the formatted form of src.
//
// No formatting rule exists yet, so the document is scanned and printed back
// unchanged. Every rule added later opts one construct out of that; anything a
// rule does not claim keeps passing through byte-identical.
func Format(src []byte) ([]byte, error) {
	doc, err := block.Scan(src)
	if err != nil {
		return nil, err
	}
	return printer.Print(doc), nil
}
