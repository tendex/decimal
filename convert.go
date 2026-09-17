package decimal

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
)

// This file converts between decimal numbers and Go's integer and binary
// floating-point types. The helpers work on sign, coefficient and exponent
// so that all three decimal formats share them.

// clampExp bounds a caller-supplied exponent so that exponent arithmetic
// cannot overflow; anything beyond the bound overflows or underflows every
// format anyway.
func clampExp(exp int) int { return min(max(exp, -maxScanExp), maxScanExp) }

// abs64 returns the sign and magnitude of v.
func abs64(v int64) (neg bool, mag uint64) {
	if v < 0 {
		return true, -uint64(v)
	}
	return false, uint64(v)
}

// toInteger rounds the finite number (-1)**neg × coef × 10**exp to an
// integer in the given direction and returns its magnitude. ok is false if
// the magnitude does not fit in 64 bits.
func toInteger(neg bool, coef uint128, exp int, mode RoundingMode) (mag uint64, inexact, ok bool) {
	switch {
	case coef.isZero():
		return 0, false, true
	case exp >= 0:
		if exp > 19 || coef.hi != 0 {
			return 0, false, false
		}
		hi, lo := bits.Mul64(coef.lo, pow10tab[exp])
		return lo, false, hi == 0
	}
	if coef.ndigits()+exp > 20 {
		return 0, false, false // at least 21 integer digits
	}
	q, rem := u256(coef).shiftRight(-exp)
	if !q.fits128() || q[1] != 0 {
		return 0, false, false
	}
	mag = q[0]
	if rem != remZero && roundsUp(mode, neg, mag&1 != 0, rem) {
		if mag++; mag == 0 {
			return 0, false, false
		}
	}
	return mag, rem != remZero, true
}

// toInt64 implements the IEEE 754 convertToInteger operations for int64.
// Invalid is raised, and the nearest int64 returned (0 for a NaN), if x is
// not finite or the rounded value is out of range.
func (c *Context) toInt64(kind kind, neg bool, coef uint128, exp int) int64 {
	if kind == finite {
		mag, inexact, ok := toInteger(neg, coef, exp, c.Rounding)
		if ok && (mag <= math.MaxInt64 || neg && mag == 1<<63) {
			if inexact {
				c.Flags |= Inexact
			}
			if neg {
				return -int64(mag)
			}
			return int64(mag)
		}
	}
	c.Flags |= Invalid
	switch {
	case kind >= quietNaN:
		return 0
	case neg:
		return math.MinInt64
	}
	return math.MaxInt64
}

// toUint64 implements the IEEE 754 convertToInteger operations for uint64.
func (c *Context) toUint64(kind kind, neg bool, coef uint128, exp int) uint64 {
	if kind == finite {
		mag, inexact, ok := toInteger(neg, coef, exp, c.Rounding)
		if ok && (!neg || mag == 0) {
			if inexact {
				c.Flags |= Inexact
			}
			return mag
		}
	}
	c.Flags |= Invalid
	if kind >= quietNaN || neg {
		return 0
	}
	return math.MaxUint64
}

// truncInt64 implements the Int64 methods: truncation toward zero,
// saturating, and reporting whether the result equals the operand.
func truncInt64(kind kind, neg bool, coef uint128, exp int) (int64, bool) {
	c := Context{Rounding: ToZero}
	v := c.toInt64(kind, neg, coef, exp)
	return v, c.Flags == 0
}

func truncUint64(kind kind, neg bool, coef uint128, exp int) (uint64, bool) {
	c := Context{Rounding: ToZero}
	v := c.toUint64(kind, neg, coef, exp)
	return v, c.Flags == 0
}

// fromFloat converts a finite binary floating-point number exactly to a
// decimal (-1)**neg × (coef + ε) × 10**exp of at most 38 digits, with sticky
// set if ε is non-zero. Rounding that to the destination format gives the
// correctly rounded conversion.
func fromFloat(f float64) (neg bool, coef uint128, exp int, sticky bool) {
	neg = math.Signbit(f)
	f = math.Abs(f)
	if f == 0 {
		return neg, uint128{}, 0, false
	}
	if f < 1<<63 && f == math.Trunc(f) {
		return neg, uint128{0, uint64(f)}, 0, false
	}
	// f = mant × 2**e2 exactly, with mant an integer.
	fr, e2 := math.Frexp(f)
	mant := uint64(fr * (1 << 53))
	e2 -= 53
	n := new(big.Int).SetUint64(mant)
	if e2 >= 0 {
		n.Lsh(n, uint(e2))
	} else {
		// mant / 2**k = mant × 5**k / 10**k
		n.Mul(n, new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(-e2)), nil))
		exp = e2
	}
	// Keep 38 digits; the rest only matter as a sticky bit. The digit
	// count estimated from the bit length may be one too large, which
	// merely keeps 37 digits.
	if drop := n.BitLen()*1233>>12 + 1 - 38; drop > 0 {
		rem := new(big.Int)
		n.QuoRem(n, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(drop)), nil), rem)
		exp += drop
		sticky = rem.Sign() != 0
	}
	var w [2]uint64
	for i, word := range n.Bits() {
		if bits.UintSize == 64 {
			w[i] = uint64(word)
		} else {
			w[i/2] |= uint64(word) << (32 * (i % 2))
		}
	}
	return neg, uint128{w[1], w[0]}, exp, sticky
}

// toFloat converts a number to the nearest float64, ties to even.
func toFloat(kind kind, neg bool, coef uint128, exp int) float64 {
	var f float64
	switch {
	case kind == infinite:
		f = math.Inf(1)
	case kind != finite:
		return math.NaN()
	case coef.isZero():
	case coef.hi == 0 && coef.lo < 1<<53 && -22 <= exp && exp <= 22:
		// Both the coefficient and the power of ten are exact float64s, so a
		// single correctly rounded multiplication or division is the
		// correctly rounded result.
		if exp >= 0 {
			f = float64(coef.lo) * float64pow10[exp]
		} else {
			f = float64(coef.lo) / float64pow10[-exp]
		}
	default:
		// strconv rounds decimal strings of any length correctly.
		var t text
		t.setCoef(coef, exp)
		var scratch [64]byte
		buf := append(scratch[:0], t.d...)
		buf = append(buf, 'e')
		buf = strconv.AppendInt(buf, int64(exp), 10)
		f, _ = strconv.ParseFloat(string(buf), 64) // ±Inf on overflow
	}
	if neg {
		f = -f
	}
	return f
}

// toFloatFlags is toFloat for the Context methods: it raises Invalid for a
// signaling NaN, Overflow and Underflow as for any rounding, and Inexact if
// the result differs from the operand.
func (c *Context) toFloatFlags(kind kind, neg bool, coef uint128, exp int) float64 {
	f := toFloat(kind, neg, coef, exp)
	switch {
	case kind == signalingNaN:
		c.Flags |= Invalid
	case kind != finite || coef.isZero():
	case math.IsInf(f, 0):
		c.Flags |= Overflow | Inexact
	case !floatEquals(math.Abs(f), coef, exp):
		c.Flags |= Inexact
		if math.Abs(f) < 0x1p-1022 {
			c.Flags |= Underflow
		}
	}
	return f
}

var float64pow10 = [...]float64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11,
	1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20, 1e21, 1e22,
}

// floatEquals reports whether the non-negative f equals coef × 10**exp.
func floatEquals(f float64, coef uint128, exp int) bool {
	if math.IsInf(f, 0) || f == 0 {
		return false // coef is non-zero
	}
	fr, e2 := math.Frexp(f)
	e2 -= 53
	// mant × 2**e2 == coef × 10**exp, cross-multiplied into integers.
	lhs := new(big.Int).SetUint64(uint64(fr * (1 << 53)))
	rhs := new(big.Int).SetUint64(coef.hi)
	rhs.Lsh(rhs, 64).Or(rhs, new(big.Int).SetUint64(coef.lo))
	if e2 >= 0 {
		lhs.Lsh(lhs, uint(e2))
	} else {
		rhs.Lsh(rhs, uint(-e2))
	}
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(max(exp, -exp))), nil)
	if exp >= 0 {
		rhs.Mul(rhs, p)
	} else {
		lhs.Mul(lhs, p)
	}
	return lhs.Cmp(rhs) == 0
}
