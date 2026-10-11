package block

import "bytes"

var ignoreFile = []byte("adocfmt: ignore-file")

// IgnoresFile reports whether a comment line above the document title says
// adocfmt: ignore-file, which leaves the file as it stands. It reads only the
// lines up to there, so it also answers for a source Parse rejects.
func IgnoresFile(src []byte) bool {
	p := newParser(src)
	p.frontMatter()
	for !p.done() {
		sh := p.shape()
		switch {
		case sh.kind == shapeBlank:
			p.at++
		case sh.kind == shapeComment:
			if isIgnoreFile(p.text(p.lines[p.at])) {
				return true
			}
			p.at++
		case sh.kind == shapeDelimiter && sh.char == '/':
			p.commentBlock()
		default:
			// Any other line, usually the title, ends the lines above it.
			return false
		}
	}
	return false
}

// isIgnoreFile reports whether a comment line says adocfmt: ignore-file.
// Spaces and tabs may stand after the slashes, and a reason after the
// directive.
func isIgnoreFile(comment []byte) bool {
	body := bytes.TrimLeft(comment[len("//"):], " \t")
	rest, ok := bytes.CutPrefix(body, ignoreFile)
	return ok && (len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t')
}
