// Package format turns AsciiDoc source into its formatted form.
package format

// Format returns the formatted form of src.
//
// No formatting rule exists yet, so this is the identity function. Every rule
// added later opts one construct out of it; anything a rule does not claim
// keeps passing through byte-identical.
func Format(src []byte) []byte {
	return src
}
