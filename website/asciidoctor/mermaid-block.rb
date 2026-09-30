# Renders a [mermaid] listing as <pre class="mermaid">, which mermaid.js turns
# into a diagram in the browser. Without it, Asciidoctor drops the style and
# the diagram shows as a plain listing.
require 'asciidoctor/extensions'

Asciidoctor::Extensions.register do
  block :mermaid do
    on_context :listing
    parse_content_as :raw
    process do |parent, reader, attrs|
      source = reader.read.gsub('&', '&amp;').gsub('<', '&lt;').gsub('>', '&gt;')
      create_pass_block parent, %(<pre class="mermaid">#{source}</pre>), attrs, subs: nil
    end
  end
end
