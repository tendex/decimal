package decimal

import "testing"

// FuzzParse checks that no input makes a parser panic, and that whatever
// parses also round-trips through String.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"0", "-0.00", "1.50", ".5", "5.", "1E+3", "1e-400", "12345678901234567890123456789012345678901234567890",
		"Inf", "-infinity", "NaN", "sNaN123", "0E+999999999999999999999", "1e", "++1", "0x1p-2", "",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if x, err := Parse32(s); err == nil {
			if y, err := Parse32(x.String()); err != nil || y.Bits() != x.Bits() {
				t.Errorf("Parse32(%q) = %v does not round-trip: %v, %v", s, x, y, err)
			}
		}
		if x, err := Parse64(s); err == nil {
			if y, err := Parse64(x.String()); err != nil || y.Bits() != x.Bits() {
				t.Errorf("Parse64(%q) = %v does not round-trip: %v, %v", s, x, y, err)
			}
			for _, format := range []byte{'e', 'f', 'g'} {
				if e := x.Exponent(); -100 < e && e < 100 {
					x.Text(format, -1)
					x.Text(format, 3)
				}
			}
		}
		if x, err := Parse128(s); err == nil {
			if y, err := Parse128(x.String()); err != nil || !sameBits128(x, y) {
				t.Errorf("Parse128(%q) = %v does not round-trip: %v, %v", s, x, y, err)
			}
		}
	})
}

// FuzzArithmetic64 checks the arithmetic of arbitrary bit patterns, including
// non-canonical ones, against the reference implementation.
func FuzzArithmetic64(f *testing.F) {
	f.Add(uint64(0x31C0000000000001), uint64(0x31C0000000000003), uint64(0), uint8(0))
	f.Add(uint64(0x6C7386F26FC0FFFF), uint64(0x77FB86F26FC0FFFF), uint64(0xB1C0000000000001), uint8(3))
	f.Add(uint64(0x7C00000000000001), uint64(0x7E00000000000002), uint64(0xF800000000000000), uint8(4))
	f.Fuzz(func(t *testing.T, a, b, c uint64, mode uint8) {
		checkArithmetic(t, rand64, RoundingMode(mode%5), New64FromBits(a), New64FromBits(b), New64FromBits(c))
	})
}

// FuzzArithmetic128 is FuzzArithmetic64 for decimal128.
func FuzzArithmetic128(f *testing.F) {
	f.Add(uint64(0x3040000000000000), uint64(1), uint64(0x3040000000000000), uint64(3), uint64(0), uint64(0), uint8(0))
	f.Add(uint64(0x0001ED09BEAD87C0), uint64(0x378D8E63FFFFFFFF), uint64(0x5FFFED09BEAD87C0), uint64(0x378D8E63FFFFFFFF),
		uint64(0xB040000000000000), uint64(7), uint8(2))
	f.Fuzz(func(t *testing.T, ah, al, bh, bl, ch, cl uint64, mode uint8) {
		checkArithmetic(t, rand128, RoundingMode(mode%5),
			New128FromBits(ah, al), New128FromBits(bh, bl), New128FromBits(ch, cl))
	})
}
