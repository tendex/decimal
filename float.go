package decimal

import (
	"math"
	"math/bits"
)

// This file converts a decimal number to the nearest float64 without going
// through its digits, in the manner of Eisel and Lemire ("Number Parsing at
// a Gigabyte per Second", Software: Practice and Experience 51, 2021), which
// is also how strconv parses short decimal strings.
//
// The idea is to multiply the coefficient by a 128-bit approximation of the
// power of ten, and to round the product. The approximation is not exact, so
// the product is not either; what makes the result trustworthy is that the
// error is bounded, which makes the exact value lie between two numbers that
// can be computed. When both of them round to the same float64, so does the
// exact value, because rounding is monotonic. When they do not, which takes
// a result within a hair of halfway between two float64s, the conversion
// gives up and the caller falls back to the exact conversion through digits.

// maxPow10Float is the exponent of the last entry of pow10float.
const maxPow10Float = minPow10Float + len(pow10float) - 1

// toFloatFast returns the float64 nearest to coef × 10**exp, rounded to
// nearest even, and whether it could determine it. It gives up, leaving the
// answer to the slower conversion through digits, when the result would be
// zero, subnormal or infinite, and when the approximation of the power of
// ten is too coarse to settle the rounding.
func toFloatFast(coef uint128, exp int) (float64, bool) {
	if coef.isZero() || exp < minPow10Float || exp > maxPow10Float {
		return 0, false
	}
	// Reduce the coefficient to the 64 bits that matter, so that the value
	// is (w + d) × 2**shift × 10**exp with 0 <= d < 1, where d is non-zero
	// exactly when sticky is set. Taking the top 64 bits of a coefficient
	// wider than that leaves w normalized already.
	w, shift, sticky := coef.lo, 0, false
	if coef.hi != 0 {
		shift = bits.Len64(coef.hi)
		w = coef.hi<<(64-shift) | coef.lo>>shift
		sticky = coef.lo<<(64-shift) != 0
	}
	if n := bits.LeadingZeros64(w); n > 0 {
		w <<= n
		shift -= n
	}

	// p approximates 10**exp from below by less than one unit in the last
	// of its 128 bits, so that the exact value lies between w×p and
	// (w + d)×(p + 1), in units of 2**(shift + p.exp - 128).
	p := &pow10float[exp-minPow10Float]
	hi, lo := bits.Mul64(w, p.hi)
	mhi, mlo := bits.Mul64(w, p.lo)
	a0 := mlo
	a1, c := bits.Add64(lo, mhi, 0)
	a2 := hi + c

	b0, c := bits.Add64(a0, w, 0)
	b1, c := bits.Add64(a1, 0, c)
	b2, carry := bits.Add64(a2, 0, c)
	if sticky {
		b0, c = bits.Add64(b0, p.lo, 0)
		b1, c = bits.Add64(b1, p.hi, c)
		b2, c = bits.Add64(b2, 0, c)
		carry |= c
		b0, c = bits.Add64(b0, 1, 0)
		b1, c = bits.Add64(b1, 0, c)
		b2, c = bits.Add64(b2, 0, c)
		carry |= c
	}
	if carry != 0 {
		return 0, false // the upper bound needs a 193rd bit
	}

	e := shift + int(p.exp) - 128
	f, ok := roundFloat192(a2, a1, a0, e)
	if !ok {
		return 0, false
	}
	if g, ok := roundFloat192(b2, b1, b0, e); !ok || g != f {
		return 0, false
	}
	return f, true
}

// roundFloat192 returns the float64 nearest to the non-zero 192-bit number
// hi:mid:lo times 2**exp, rounded to nearest even, and whether that number
// is a normal float64. Subnormal and infinite results are left to the slower
// conversion. The caller guarantees that hi is non-zero.
func roundFloat192(hi, mid, lo uint64, exp int) (float64, bool) {
	// The exponent of the most significant bit of the number.
	n := bits.Len64(hi)
	e := exp + 128 + n - 1
	if e < -1022 || e > 1023 {
		return 0, false
	}
	// Keep 53 bits, and round to nearest even on what is dropped.
	drop := uint(n - 53)
	m := hi >> drop
	rem := hi & (1<<drop - 1)
	half := uint64(1) << (drop - 1)
	if rem > half || rem == half && (mid|lo != 0 || m&1 != 0) {
		m++
		if m == 1<<53 {
			m >>= 1
			if e++; e > 1023 {
				return 0, false
			}
		}
	}
	return math.Float64frombits(uint64(e+1023)<<52 | m&(1<<52-1)), true
}
