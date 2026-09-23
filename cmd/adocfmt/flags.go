package main

import (
	"flag"
	"fmt"
	"io"
)

// options holds what the flags set.
type options struct {
	check   bool
	version bool
	help    bool
}

// flagEntry is one line of the command line. Registration and the usage text
// both read the same entries, so a flag cannot reach one and miss the other.
type flagEntry struct {
	target      *bool
	short, long string
	help        string
}

func (o *options) entries() []flagEntry {
	return []flagEntry{
		{&o.check, "c", "check", "Name the files that are not formatted; exit 1 if any."},
		{&o.version, "", "version", "Print the version."},
		{&o.help, "h", "help", "Print this help."},
	}
}

// register binds the flags to o.
//
// The flag package writes usage and errors to one stream, and the command sends
// them to different ones, so it prints both itself.
func (o *options) register() *flag.FlagSet {
	flags := flag.NewFlagSet("adocfmt", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for _, entry := range o.entries() {
		flags.BoolVar(entry.target, entry.long, false, entry.help)
		if entry.short != "" {
			flags.BoolVar(entry.target, entry.short, false, entry.help)
		}
	}
	return flags
}

// usage writes what a reader who asked for it came for, which is why it goes to
// stdout. A short and a long name are one entry, which is what flag.PrintDefaults
// cannot do.
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, "adocfmt formats AsciiDoc files.\n\nUsage:\n  adocfmt [flags] [path ...]\n\nFlags:\n")
	for _, entry := range (&options{}).entries() {
		name := "    --" + entry.long
		if entry.short != "" {
			name = "-" + entry.short + ", --" + entry.long
		}
		_, _ = fmt.Fprintf(w, "  %-16s %s\n", name, entry.help)
	}
	_, _ = fmt.Fprint(w, "\nWith no path adocfmt reads stdin and writes the result to stdout.\n")
}
