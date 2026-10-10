# Renders one document per line for render.go: it reads the source as a JSON
# string and writes a JSON object holding the HTML or the error. The options
# are those of
#
#   asciidoctor --embedded --attribute showtitle --safe-mode safe -
require 'asciidoctor'
require 'json'

$stdout.sync = true
Asciidoctor::LoggerManager.logger = Asciidoctor::NullLogger.new

$stdin.each_line do |line|
  html = Asciidoctor.convert JSON.parse(line), safe: :safe, standalone: false, attributes: { 'showtitle' => '' }
  puts JSON.generate({ html: html })
rescue StandardError => e
  puts JSON.generate({ error: "#{e.class}: #{e.message}" })
end
