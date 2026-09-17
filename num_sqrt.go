package decimal

import (
	"math"
	"math/bits"
)

// isqrt128 returns floor(sqrt(n)) for n < 2**126.
func isqrt128(n uint128) uint64 {
	// A float64 estimate is good to about 52 bits; one Newton step makes it
	// good to within one, and the loops below settle the rest.
	s := uint64(math.Sqrt(float64(n.hi)*0x1p64 + float64(n.lo)))
	if n.hi != 0 {
		q, _ := n.quoRem64(s)
		s = (s + q.lo) / 2
	}
	for {
		hi, lo := bits.Mul64(s, s)
		if !n.less(uint128{hi, lo}) {
			break
		}
		s--
	}
	for {
		hi, lo := bits.Mul64(s+1, s+1)
		if n.less(uint128{hi, lo}) {
			break
		}
		s++
	}
	return s
}

// sqrt returns the square root of x.
func (c *Context) sqrt(f *format, x num) num {
	switch {
	case x.isNaN():
		return c.nan(f, x, x)
	case x.isZero():
		x.exp >>= 1 // sqrt(-0) is -0
		return x
	case x.neg:
		return c.invalid()
	case x.kind == infinite:
		return x
	}
	// Scale the coefficient to 2p+1 or 2p+2 digits, whichever leaves an
	// even exponent, so that its integer square root has p+1 digits:
	// rounding then always discards a digit, and a sticky bit for the
	// remainder is all it needs to be correct.
	k := 2*f.prec + 2 - ndigits64(x.coef)
	if (int(x.exp)-k)&1 != 0 {
		k--
	}
	n := pow10tab128[k].mul64(x.coef)
	s := isqrt128(n)
	hi, lo := bits.Mul64(s, s)
	exact := n == uint128{hi, lo}
	exp := (int(x.exp) - k) / 2
	if exact {
		// Move back toward the preferred exponent, floor(x.exp / 2).
		var z int
		s, z = trailingZeros64(s, int(x.exp>>1)-exp)
		exp += z
	}
	return c.round(f, false, uint128{0, s}, exp, !exact)
}
