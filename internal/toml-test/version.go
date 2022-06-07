//go:build go1.16
// +build go1.16

package tomltest

type versionSpec struct {
	inherit string
	exclude []string
}

var versions = map[string]versionSpec{
	"next": versionSpec{},

	"1.0.0": versionSpec{
		exclude: []string{
			"valid/string/escape-esc", // \e
		},
	},

	// Added in 1.0.0:
	//   Leading zeroes in exponent parts of floats are permitted.
	//   Allow raw tab characters in basic strings and multi-line basic strings.
	//   Allow heterogenous values in arrays.
	"0.5.0": versionSpec{
		inherit: "1.0.0",
		exclude: []string{
			"valid/hetergeneous",
			"valid/array/mixed-*",
		},
	},

	// Added in 0.5.0:
	//   Add dotted keys.
	//   Add hex, octal, and binary integer formats.
	//   Add special float values (inf, nan)
	//   Add Local Date-Time.
	//   Add Local Date.
	//   Add Local Time.
	//   Allow space (instead of T) to separate date and time in Date-Time.
	//   Allow accidental whitespace between backslash and newline in the line
	//   continuation operator in multi-line basic strings.
	"0.4.0": versionSpec{
		inherit: "0.5.0",
		exclude: []string{
			"valid/datetime/local*",
			"valid/key/dotted",
		},
	},
}
