//go:build go1.16
// +build go1.16

package tomltest

type versionSpec struct {
	inherit string
	exclude []string
}

var versions = map[string]versionSpec{
	"next": versionSpec{
		exclude: []string{
			"invalid/datetime/no-secs",          // Times without seconds is no longer invalid.
			"invalid/string/basic-byte-escapes", // \x is now valid.
			"invalid/inline-table/trailing-comma",
			"invalid/inline-table/linebreak-1",
			"invalid/inline-table/linebreak-2",
			"invalid/inline-table/linebreak-3",
			"invalid/inline-table/linebreak-4",
		},
	},

	"1.0.0": versionSpec{
		exclude: []string{
			"valid/string/escape-esc",                               // \e
			"valid/string/hex-escape", "invalid/string/bad-hex-esc", // \x..
			"valid/datetime/no-seconds", // Times without seconds
			"valid/inline-table/newline",
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
