package toml

import "testing"

func TestParseOversizedHexLiteral(t *testing.T) {
	hash := "6501bc346e2ad6eecae3bcf7eee78ac21b1a03bdc4fda879a4d24793c1f08db7"
	p, err := parse("x = 0x"+hash, 128)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := p.mapping["x"].([]byte)
	if !ok {
		t.Fatalf("got %T", p.mapping["x"])
	}
	if len(b) != 32 {
		t.Fatalf("len=%d", len(b))
	}
}

func TestTryParseOversizedHexInteger(t *testing.T) {
	hash := "6501bc346e2ad6eecae3bcf7eee78ac21b1a03bdc4fda879a4d24793c1f08db7"
	b, ok := tryParseOversizedHexInteger("0x" + hash)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(b) != 32 {
		t.Fatalf("len=%d", len(b))
	}
}

func TestDecodeHexBytesString(t *testing.T) {
	tests := []struct {
		in   string
		want []byte
		err  bool
	}{
		{"deadbeef", []byte{0xde, 0xad, 0xbe, 0xef}, false},
		{"0xdeadbeef", []byte{0xde, 0xad, 0xbe, 0xef}, false},
		{"0X01", []byte{0x01}, false},
		{"abc", nil, true},
		{"ghij", nil, true},
	}
	for _, tt := range tests {
		got, err := decodeHexBytes(tt.in)
		if (err != nil) != tt.err {
			t.Errorf("decodeHexBytes(%q): err=%v wantErr=%v", tt.in, err, tt.err)
			continue
		}
		if !tt.err && string(got) != string(tt.want) {
			t.Errorf("decodeHexBytes(%q) = %x, want %x", tt.in, got, tt.want)
		}
	}
}
