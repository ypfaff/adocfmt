package block

import "bytes"

var ignoreFile = []byte("adocfmt: ignore-file")

// IgnoresFile reports whether a comment line above the document title says
// adocfmt: ignore-file, which leaves the file as it stands. It reads only the
// lines up to there, so it also answers for a source Parse rejects.
func IgnoresFile(src []byte) bool {
	p := &parser{src: src, lines: splitLines(src, bodyStart(src))}
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
			p.skipTopCommentBlock()
		default:
			// Any other line, usually the title, ends the lines above it.
			return false
		}
	}
	return false
}

// skipTopCommentBlock moves past the comment block that opens on the current
// line. Whether its closing line exists is left to Parse to report.
func (p *parser) skipTopCommentBlock() {
	closer := p.text(p.lines[p.at])
	p.at++
	p.skipVerbatim(closer, true)
	if !p.done() {
		p.at++
	}
}

// isIgnoreFile reports whether a comment line says adocfmt: ignore-file.
// Spaces and tabs may stand after the slashes, and a reason after the
// directive.
func isIgnoreFile(comment []byte) bool {
	body := bytes.TrimLeft(comment[len("//"):], " \t")
	rest, ok := bytes.CutPrefix(body, ignoreFile)
	return ok && (len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t')
}
