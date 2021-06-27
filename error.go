package toml

import (
	"fmt"
	"strings"
)

// ParseError is used when there is an error decoding TOML data.
//
// For example invalid TOML syntax, duplicate keys, etc.
type ParseError struct {
	Message  string   // Short technical message.
	Usage    string   // Longer message with usage guidance; may be blank.
	Position Position // Position of the error
	LastKey  string   // Last parsed key, may be blank.

	err   error
	input string
}

// Position of an error.
type Position struct {
	Line  int // Line number, starting at 1.
	Start int // Start of error, as byte offset starting at 0.
	Len   int // Lenght in bytes.
}

func (pe ParseError) Error() string {
	msg := pe.Message
	if msg == "" { // TODO: temporary
		msg = pe.err.Error()
	}

	if pe.LastKey == "" {
		return fmt.Sprintf("toml: line %d: %s", pe.Position.Line, msg)
	}
	return fmt.Sprintf("toml: line %d (last key %q): %s",
		pe.Position.Line, pe.LastKey, msg)
}

func (pe ParseError) ExtendedWithUsage() string {
	m := pe.Extended()
	if u, ok := pe.err.(interface{ Usage() string }); ok {
		return m + "\n" + strings.TrimSpace(u.Usage()) + "\n"
	}
	return m
}

func (pe ParseError) Extended() string {
	if pe.input == "" { // Should never happen, but just in case.
		return pe.Error()
	}

	var (
		lines = strings.Split(pe.input, "\n")
		col   = pe.column(lines)
		b     = new(strings.Builder)
	)

	msg := pe.Message
	if msg == "" {
		msg = pe.err.Error()
	}

	// TODO: don't show control characters as literals.

	fmt.Fprintf(b, "toml: error: %s\n\nAt line %d, column %d-%d:\n\n",
		msg, pe.Position.Line, col, col+pe.Position.Len)
	if pe.Position.Line > 2 {
		fmt.Fprintf(b, "% 7d | %s\n", pe.Position.Line-2, lines[pe.Position.Line-3])
	}
	if pe.Position.Line > 1 {
		fmt.Fprintf(b, "% 7d | %s\n", pe.Position.Line-1, lines[pe.Position.Line-2])
	}
	fmt.Fprintf(b, "% 7d | %s\n", pe.Position.Line, lines[pe.Position.Line-1])
	fmt.Fprintf(b, "% 10s%s%s\n", "", strings.Repeat(" ", col), strings.Repeat("^", pe.Position.Len))
	return b.String()
}

func (pe ParseError) column(lines []string) int {
	var pos, col int
	for i := range lines {
		ll := len(lines[i]) + 1 // +1 for the removed newline
		if pos+ll >= pe.Position.Start {
			col = pe.Position.Start - pos
			if col < 0 { // Should never happen, but just in case.
				col = 0
			}
			break
		}
		pos += ll
	}

	return col
}

type (
	errLexEscape      struct{ r rune }
	errLexUTF8        struct{ b byte }
	errLexInvalidNum  struct{ v string }
	errLexInvalidDate struct{ v string }
)

func (e errLexEscape) Error() string      { return fmt.Sprintf(`invalid escape in string '\%c'`, e.r) }
func (e errLexEscape) Usage() string      { return usageEscape }
func (e errLexUTF8) Error() string        { return fmt.Sprintf("invalid UTF-8 byte: 0x%02x", e.b) }
func (e errLexUTF8) Usage() string        { return "" }
func (e errLexInvalidNum) Error() string  { return fmt.Sprintf("invalid number: %q", e.v) }
func (e errLexInvalidNum) Usage() string  { return "" }
func (e errLexInvalidDate) Error() string { return fmt.Sprintf("invalid date: %q", e.v) }
func (e errLexInvalidDate) Usage() string { return "" }

const usageEscape = `
A '\' inside a "-delimited string is interpreted as an escape character.

The following escape sequences are supported:
\b, \t, \n, \f, \r, \", \\, \uXXXX, and \UXXXXXXXX

To prevent a '\' from being recognized as an escape character, use:

- a ' or '''-delimited string; escape characters aren't processed in them; or
- two backslashes to get a single backslash: '\\'.

If you're trying to add a Windows path (e.g. "C:\Users\martin") then using '/'
instead of '\' will usually also work: "C:/Users/martin".
`
