package toml

import (
	"fmt"
	"strings"
)

// ParseError is used when there is an error decoding TOML data.
//
// For example invalid TOML syntax, duplicate keys, etc.
type ParseError struct {
	Message  string
	Position Position
	LastKey  string // Last parsed key, may be blank.
	Input    string
}

func (pe ParseError) Error() string {
	if pe.LastKey == "" {
		return fmt.Sprintf("toml: %s: %s", pe.Position, pe.Message)
	}
	return fmt.Sprintf("toml: %s (last key parsed '%s'): %s",
		pe.Position, pe.LastKey, pe.Message)
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

// ––– array-mixed-types-arrays-and-ints.toml –––––––––––––––––––––––––––
// toml: error: Array contains values of type 'Integer' and 'Array', but arrays must be homogeneous.
//              at line 1; column 1-15; byte offset 15
//              last key parsed was "arrays-and-ints"
//
//      1 | arrays-and-ints =  [1, ["Arrays are not integers."]]
//         ^^^^^^^^^^^^^^^
//
// This is on the key as the parser doesn't use the lex position.
func (pe ParseError) ExtError() string {
	if pe.Input == "" {
		return pe.Error()
	}

	lines := strings.Split(pe.Input, "\n")
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

	b := new(strings.Builder)
	//fmt.Fprintf(b, "toml: error on line %d: %s\n", line, pe.Message)
	fmt.Fprintf(b, "toml: error: %s\n", pe.Message)
	//fmt.Fprintf(b, "             on line %d", pe.Position.Line)
	fmt.Fprintf(b, "             %s\n", pe.Position)
	if pe.LastKey != "" {
		fmt.Fprintf(b, "             last key parsed was %q", pe.LastKey)
	}
	b.WriteString("\n\n")

	if pe.Position.Line > 2 {
		fmt.Fprintf(b, "% 6d | %s\n", pe.Position.Line-2, lines[pe.Position.Line-3])
	}
	if pe.Position.Line > 1 {
		fmt.Fprintf(b, "% 6d | %s\n", pe.Position.Line-1, lines[pe.Position.Line-2])
	}

	l := pe.Position.Len - 1
	if l < 0 {
		l = 0
	}

	fmt.Fprintf(b, "% 6d | %s\n", pe.Position.Line, lines[pe.Position.Line-1])
	fmt.Fprintf(b, "% 9s%s%s\n", "",
		strings.Repeat(" ", col),
		strings.Repeat("^", l+1))

	// if len(lines)-1 > pe.Position.Line && lines[pe.Position.Line+1] != "" {
	// 	fmt.Fprintf(b, "% 6d | %s\n", pe.Position.Line+1, lines[pe.Position.Line+1])
	// }

	return b.String()
}
