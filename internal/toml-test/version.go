package tomltest

type versionSpec struct {
	inherit string
	exclude []string
}

var versions = map[string]versionSpec{
	"1.1.0": versionSpec{
		exclude: []string{
			"invalid/datetime/no-secs", // Times without seconds is no longer invalid.
			"invalid/local-time/no-secs",
			"invalid/local-datetime/no-secs",
			"invalid/string/basic-byte-escapes", // \x is now valid.
			"invalid/inline-table/trailing-comma",
			"invalid/inline-table/linebreak-1",
			"invalid/inline-table/linebreak-2",
			"invalid/inline-table/linebreak-3",
			"invalid/inline-table/linebreak-4",
			"invalid/key/special-character", // Unicode can now be in bare keys.
		},
	},

	"1.0.0": versionSpec{
		exclude: []string{
			"valid/string/escape-esc",                               // \e
			"valid/string/hex-escape", "invalid/string/bad-hex-esc", // \x..
			"valid/datetime/no-seconds", // Times without seconds
			"valid/inline-table/newline",
			"valid/key/unicode", // Unicode bare keys
		},
	},
}
