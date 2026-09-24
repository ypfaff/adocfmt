package main

import (
	"flag"
	"fmt"
	"io"
)

type options struct {
	write   bool
	check   bool
	version bool
	help    bool
}

// Registration and the usage text both read these, so a flag cannot reach one
// and miss the other.
type flagEntry struct {
	target      *bool
	short, long string
	help        string
}

func (o *options) entries() []flagEntry {
	return []flagEntry{
		{&o.write, "w", "write", "Write the result back to the file."},
		{&o.check, "c", "check", "Name the files that are not formatted; exit 1 if any."},
		{&o.version, "", "version", "Print the version."},
		{&o.help, "h", "help", "Print this help."},
	}
}

func (o *options) register() *flag.FlagSet {
	flags := flag.NewFlagSet("adocfmt", flag.ContinueOnError)
	// The package writes usage and errors to one stream, and the command sends
	// them to different ones, so it prints both itself.
	flags.SetOutput(io.Discard)
	for _, entry := range o.entries() {
		flags.BoolVar(entry.target, entry.long, false, entry.help)
		if entry.short != "" {
			flags.BoolVar(entry.target, entry.short, false, entry.help)
		}
	}
	return flags
}

const usageHeader = `adocfmt formats AsciiDoc files.

Usage:
  adocfmt [flags] [path ...]

Flags:
`

const usageFooter = `
With no path adocfmt reads stdin and writes the result to stdout.
`

// usage goes to stdout, because a reader asked for it. A short and a long name
// are one entry, which flag.PrintDefaults cannot do.
func usage(w io.Writer) {
	_, _ = io.WriteString(w, usageHeader)
	for _, entry := range (&options{}).entries() {
		name := "    --" + entry.long
		if entry.short != "" {
			name = "-" + entry.short + ", --" + entry.long
		}
		_, _ = fmt.Fprintf(w, "  %-16s %s\n", name, entry.help)
	}
	_, _ = io.WriteString(w, usageFooter)
}
