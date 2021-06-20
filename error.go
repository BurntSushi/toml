package toml

import (
	"fmt"
	"strings"
)

// ParseError is used when there is an error decoding TOML data.
//
// For example invalid TOML syntax, duplicate keys, etc.
type ParseError struct {
	Message string
	Line    int
	Pos     int    // Byte offset
	LastKey string // Last parsed key, may be blank.
	Input   string
}

func (pe ParseError) Error() string {
	if pe.LastKey == "" {
		return fmt.Sprintf("toml: line %d: %s", pe.Line, pe.Message)
	}
	return fmt.Sprintf("toml: line %d (last key parsed '%s'): %s",
		pe.Line, pe.LastKey, pe.Message)
}

// Clang error:
//
// a.c:2:9: warning: incompatible pointer to integer conversion returning 'char [4]' from a function with result type 'int' [-Wint-conversion]
//         return "zxc";
//                ^~~~~
// 1 warning generated.
//
// Rust:
//
// error[E0425]: cannot find value `err` in this scope
//    --> a.rs:3:5
//     |
// 3   |     err
//     |     ^^^ help: a tuple variant with a similar name exists: `Err`
//
// error: aborting due to previous error
//
// For more information about this error, try `rustc --explain E0425`.

func (pe ParseError) ExtError() string {
	if pe.Input == "" {
		return pe.Error()
	}

	lines := strings.Split(pe.Input, "\n")
	var line, pos, col int
	for i := range lines {
		ll := len(lines[i]) + 1 // +1 for the removed newline
		if pos+ll >= pe.Pos {
			line = i
			col = pe.Pos - pos - 1
			if col < 0 { // Should never happen, but just in case.
				col = 0
			}
			break
		}
		pos += ll
	}

	b := new(strings.Builder)
	//fmt.Fprintf(b, "toml: error on line %d: %s\n", line, pe.Message)
	fmt.Fprintf(b, "toml: error: %s\n", pe.Message)
	fmt.Fprintf(b, "             on line %d", line+1)
	if pe.LastKey != "" {
		fmt.Fprintf(b, "; last key parsed was %q", pe.LastKey)
	}
	b.WriteString("\n\n")

	if line > 1 {
		fmt.Fprintf(b, "% 6d | %s\n", line-1, lines[line-2])
	}
	if line > 0 {
		fmt.Fprintf(b, "% 6d | %s\n", line, lines[line-1])
	}

	fmt.Fprintf(b, "% 6d | %s\n", line+1, lines[line])
	fmt.Fprintf(b, "% 9s%s^\n", "", strings.Repeat(" ", col))

	// if len(lines)-1 > line && lines[line+1] != "" {
	// 	fmt.Fprintf(b, "% 6d | %s\n", line+1, lines[line+1])
	// }

	return b.String()
}
