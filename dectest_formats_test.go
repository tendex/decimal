package decimal

import (
	"fmt"
	"strconv"
)

// The format-specific parts of the decTest adapters. The adapters proper
// are in dectest64_test.go, from which gen_formats.go derives the other two.

func bool01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// hexArg converts an operand given as '#' and the hexadecimal DPD encoding.

func hexArg32(s string) (Decimal32, bool) {
	b, err := strconv.ParseUint(s[1:], 16, 32)
	return New32FromDPD(uint32(b)), err == nil && len(s) == 9
}

func hexArg64(s string) (Decimal64, bool) {
	b, err := strconv.ParseUint(s[1:], 16, 64)
	return New64FromDPD(b), err == nil && len(s) == 17
}

func hexArg128(s string) (Decimal128, bool) {
	if len(s) != 33 {
		return Decimal128{}, false
	}
	hi, err1 := strconv.ParseUint(s[1:17], 16, 64)
	lo, err2 := strconv.ParseUint(s[17:], 16, 64)
	return New128FromDPD(hi, lo), err1 == nil && err2 == nil
}

// hex returns the DPD encoding of a result string, which identifies the
// result exactly, in the form decTest writes it.

func hex32(s string) string { return fmt.Sprintf("#%08x", MustParse32(s).DPD()) }

func hex64(s string) string { return fmt.Sprintf("#%016x", MustParse64(s).DPD()) }

func hex128(s string) string {
	hi, lo := MustParse128(s).DPD()
	return fmt.Sprintf("#%016x%016x", hi, lo)
}

// zeroLike returns a zero with the exponent of x.

func zeroLike32(x Decimal32) Decimal32 { return pack32(num{exp: x.unpack().exp}) }

func zeroLike64(x Decimal64) Decimal64 { return pack64(num{exp: x.unpack().exp}) }

func zeroLike128(x Decimal128) Decimal128 { return pack128(num128{exp: x.unpack().exp}) }

func scaleArg128(y Decimal128) (int, bool) {
	n := y.unpack()
	return scaleArgNum(num{coef: n.coef.lo, exp: n.exp, neg: n.neg, kind: n.kind}, 2*(6144+34))
}

// scaleArg converts the second operand of scaleb, which General Decimal
// Arithmetic requires to be an integer with a zero exponent and magnitude at
// most 2 × (Emax + precision).

func scaleArgNum(y num, limit uint64) (int, bool) {
	if y.kind != finite || y.exp != 0 || y.coef > limit {
		return 0, false
	}
	if y.neg {
		return -int(y.coef), true
	}
	return int(y.coef), true
}

func scaleArg32(y Decimal32) (int, bool) { return scaleArgNum(y.unpack(), 2*(96+7)) }

func scaleArg64(y Decimal64) (int, bool) { return scaleArgNum(y.unpack(), 2*(384+16)) }
