# Prints Asciidoctor::VERSION, then for each line on stdin, a JSON string, the
# kind of block Asciidoctor opens with it. It asks what Asciidoctor asks of the
# first line of a block in a section body, in the same order, through its own
# methods:
#
# directive    PreprocessorReader#process_line consumes or replaces the line.
#              An escaped directive loses its backslash and reads on.
# bad-directive
#              A directive it logs as malformed.
# anchor, attributes, title, comment, attr-entry, delimiter of a comment block
#              Parser.parse_block_metadata_line takes the line; the chars it
#              starts with name the branch that took it.
# heading      Parser.atx_section_title?, which Parser.next_section asks next.
# delimiter    Parser.is_delimited_block?, the first question of next_block.
# break, macro, marker, indented, quote, text
#              The context of the block Parser.next_block builds.
require 'asciidoctor'
require 'json'

METADATA = { '[[' => 'anchor', '[' => 'attributes', '.' => 'title', '////' => 'delimiter', '/' => 'comment',
             ':' => 'attr-entry' }.freeze

CONTEXTS = { paragraph: 'text', admonition: 'text', literal: 'indented', quote: 'quote',
             thematic_break: 'break', page_break: 'break', image: 'macro', video: 'macro', audio: 'macro',
             toc: 'macro', ulist: 'marker', olist: 'marker', colist: 'marker', dlist: 'marker' }.freeze

def kind(line)
  # A fresh document per line, since an attribute entry changes how the lines
  # after it read. It is a parsed one, since an unparsed one reads an entry the
  # way the header does.
  doc = Asciidoctor.load '', safe: :safe
  Asciidoctor::LoggerManager.logger = logger = Asciidoctor::MemoryLogger.new
  # Stripped the way Document strips every source line.
  line = Asciidoctor::Helpers.prepare_source_array([line])[0]
  if (read = Asciidoctor::PreprocessorReader.new(doc, [line]).peek_line) != line
    unless line.start_with?('\\') && read == line.slice(1..)
      return logger.messages.any? { |m| m[:message].to_s.include? 'malformed' } ? 'bad-directive' : 'directive'
    end

    line = read
  end
  if Asciidoctor::Parser.parse_block_metadata_line Asciidoctor::Reader.new([line]), doc, {}
    return METADATA.find { |head, _| line.start_with? head }[1]
  end
  return 'heading' if Asciidoctor::Parser.atx_section_title? line
  return 'delimiter' if Asciidoctor::Parser.is_delimited_block? line

  CONTEXTS.fetch Asciidoctor::Parser.next_block(Asciidoctor::Reader.new([line]), doc).context
end

puts Asciidoctor::VERSION
$stdin.each_line { |json| puts kind(JSON.parse(json)) }
