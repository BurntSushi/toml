package toml

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// decodeHexBytes decodes a hexadecimal string into bytes. An optional "0x" or
// "0X" prefix is stripped. The string must contain an even number of hex digits.
func decodeHexBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(s) == 0 {
		return nil, nil
	}
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("toml: odd number of hex digits in %q", s)
	}
	for _, r := range s {
		if !isHex(r) {
			return nil, fmt.Errorf("toml: invalid hex digit in %q", s)
		}
	}
	return hex.DecodeString(s)
}

// tryParseOversizedHexInteger parses a 0x-prefixed integer literal that does
// not fit in int64 as a byte slice instead.
func tryParseOversizedHexInteger(val string) ([]byte, bool) {
	lower := strings.ToLower(val)
	if !strings.HasPrefix(lower, "0x") {
		return nil, false
	}
	if len(lower) <= 2+16 {
		return nil, false
	}
	b, err := decodeHexBytes(val)
	if err != nil {
		return nil, false
	}
	return b, true
}
