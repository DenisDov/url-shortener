package base62

import "testing"

func TestEncodeDecode(t *testing.T) {
	cases := []uint64{0, 1, 61, 62, 12345, 999999999}

	for _, n := range cases {
		encoded := Encode(n)
		decoded := Decode(encoded)
		if decoded != n {
			t.Errorf("Encode(%d) = %q, Decode(%q) = %d, want %d", n, encoded, encoded, decoded, n)
		}
	}
}

func TestRandomCode(t *testing.T) {
	code, err := RandomCode(7)
	if err != nil {
		t.Fatalf("RandomCode returned error: %v", err)
	}
	if len(code) != 7 {
		t.Errorf("expected length 7, got %d (%q)", len(code), code)
	}

	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		c, err := RandomCode(7)
		if err != nil {
			t.Fatalf("RandomCode returned error: %v", err)
		}
		seen[c] = true
	}
	if len(seen) < 990 {
		t.Errorf("expected near-unique codes across 1000 generations, got %d unique", len(seen))
	}
}
