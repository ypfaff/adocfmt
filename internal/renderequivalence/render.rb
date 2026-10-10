# Renders one document per line for render.go: it reads the source as a JSON
# string and writes a JSON object holding the HTML or the error. The options
# are those of
#
#   asciidoctor --attribute reproducible --safe-mode safe -
require 'asciidoctor'
require 'json'

$stdout.sync = true
Asciidoctor::LoggerManager.logger = Asciidoctor::NullLogger.new

$stdin.each_line do |line|
  # reproducible keeps the time of the render out of the footer.
  html = Asciidoctor.convert JSON.parse(line), safe: :safe, standalone: true, attributes: { 'reproducible' => '' }
  puts JSON.generate({ html: html })
rescue StandardError => e
  puts JSON.generate({ error: "#{e.class}: #{e.message}" })
end
