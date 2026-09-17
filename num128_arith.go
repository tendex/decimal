package decimal

import (
	"math"
	"math/bits"
)

func (n num128) wide() wide { return wide{coef: u256(n.coef), exp: int(n.exp), neg: n.neg} }

// scale returns coef × 10**n, which the caller guarantees fits in 128 bits.
func (coef uint128) scale(n int) uint128 {
	if n <= 19 {
		return coef.mul64(pow10tab[n])
	}
	return coef.mul(pow10tab128[n]).low128()
}

// add128 returns x + y.
func (c *Context) add128(x, y num128) num128 {
	if x.kind|y.kind != finite {
		switch {
		case x.isNaN() || y.isNaN():
			return c.nan128(x, y)
		case x.kind == infinite && y.kind == infinite && x.neg != y.neg:
			return c.invalid128() // Inf - Inf
		case x.kind == infinite:
			return num128{kind: infinite, neg: x.neg}
		}
		return num128{kind: infinite, neg: y.neg}
	}
	// Let x be the operand with the larger exponent. The preferred exponent
	// of the sum is the smaller one, y.exp.
	if x.exp < y.exp {
		x, y = y, x
	}
	if x.coef.isZero() {
		// 0 + y is y exactly, at the preferred exponent.
		if y.coef.isZero() && x.neg != y.neg {
			y.neg = c.zeroSign()
		}
		return y
	}
	gap := int(x.exp - y.exp)
	if gap+x.coef.ndigits() > 38 {
		return c.roundWide128(c.addWide(x.wide(), y.wide()))
	}
	// x can be aligned to y.exp within 128 bits; the sum is exact.
	xc := x.coef.scale(gap)
	var sum uint128
	neg := x.neg
	switch {
	case x.neg == y.neg:
		sum = xc.add(y.coef)
	case y.coef.less(xc):
		sum = xc.sub(y.coef)
	case xc.less(y.coef):
		sum, neg = y.coef.sub(xc), y.neg
	default:
		neg = c.zeroSign()
	}
	return c.round128(neg, sum, int(y.exp), false)
}

// sub128 returns x - y.
func (c *Context) sub128(x, y num128) num128 {
	if !y.isNaN() {
		y.neg = !y.neg
	}
	return c.add128(x, y)
}

// mul128 returns x × y.
func (c *Context) mul128(x, y num128) num128 {
	neg := x.neg != y.neg
	if x.kind|y.kind != finite {
		switch {
		case x.isNaN() || y.isNaN():
			return c.nan128(x, y)
		case x.isZero() || y.isZero():
			return c.invalid128() // 0 × Inf
		}
		return num128{kind: infinite, neg: neg}
	}
	exp := int(x.exp) + int(y.exp)
	if x.coef.hi|y.coef.hi == 0 {
		hi, lo := bits.Mul64(x.coef.lo, y.coef.lo)
		return c.round128(neg, uint128{hi, lo}, exp, false)
	}
	return c.roundWide128(wide{coef: x.coef.mul(y.coef), exp: exp, neg: neg})
}

// quo128 returns x / y.
func (c *Context) quo128(x, y num128) num128 {
	neg := x.neg != y.neg
	if x.kind|y.kind != finite {
		switch {
		case x.isNaN() || y.isNaN():
			return c.nan128(x, y)
		case x.kind == infinite && y.kind == infinite:
			return c.invalid128() // Inf / Inf
		case x.kind == infinite:
			return num128{kind: infinite, neg: neg}
		}
		return num128{exp: emin128, neg: neg} // finite / Inf
	}
	if y.coef.isZero() {
		if x.coef.isZero() {
			return c.invalid128() // 0 / 0
		}
		c.Flags |= DivisionByZero
		return num128{kind: infinite, neg: neg}
	}
	exp := int(x.exp) - int(y.exp) // preferred exponent
	if x.coef.isZero() {
		return c.round128(neg, uint128{}, exp, false)
	}
	// Scale x so that the integer quotient has prec+1 or prec+2 digits; see
	// quo. The scaled dividend has at most 69 digits and the quotient 36.
	k := prec128 + 1 + y.coef.ndigits() - x.coef.ndigits()
	q256, r := u256(x.coef).mulPow10(k).quoRem128(y.coef)
	q := q256.low128()
	exp -= k
	if r.isZero() {
		// Exact: move back toward the preferred exponent.
		var z int
		q, z = trailingZeros128(q, k)
		exp += z
	}
	return c.round128(neg, q, exp, !r.isZero())
}

// fma128 returns x × y + z, rounded once.
func (c *Context) fma128(x, y, z num128) num128 {
	neg := x.neg != y.neg
	if x.kind|y.kind|z.kind != finite {
		switch {
		case x.isNaN() || y.isNaN() || z.isNaN():
			return c.nan3128(x, y, z)
		case x.kind == infinite || y.kind == infinite:
			if x.isZero() || y.isZero() {
				return c.invalid128() // 0 × Inf
			}
			if z.kind == infinite && z.neg != neg {
				return c.invalid128() // Inf - Inf
			}
			return num128{kind: infinite, neg: neg}
		}
		return z // finite × finite + Inf
	}
	p := wide{coef: x.coef.mul(y.coef), exp: int(x.exp) + int(y.exp), neg: neg}
	return c.roundWide128(c.addWide(p, z.wide()))
}

// isqrt256 returns floor(sqrt(n)) for n < 2**250.
func isqrt256(n uint256) uint128 {
	// Estimate from the top 64 bits; each Newton step then doubles the
	// number of correct bits, and the loops below settle the last one.
	shift := max(n.bitLen()-63, 0) &^ 1 // even, so the root shifts by half
	top := n
	for i := 0; i < shift/64; i++ {
		top = uint256{top[1], top[2], top[3], 0}
	}
	if s := uint(shift % 64); s != 0 {
		top[0] = top[0]>>s | top[1]<<(64-s)
	}
	fr, e := math.Frexp(math.Sqrt(float64(top[0])))
	s := uint128{0, uint64(fr * (1 << 53))}
	if e += shift/2 - 53; e >= 0 {
		s = s.lsh(uint(e))
	} else {
		s = s.rsh(uint(-e))
	}
	if s.isZero() {
		s.lo = 1
	}
	for i := 0; i < 2; i++ {
		q, _ := n.quoRem128(s)
		s = s.add(q.low128()).rsh(1)
	}
	for n.cmp(s.mul(s)) < 0 {
		s = s.sub64(1)
	}
	for {
		t := s.add64(1)
		if n.cmp(t.mul(t)) < 0 {
			return s
		}
		s = t
	}
}

// sqrt128 returns the square root of x; see sqrt.
func (c *Context) sqrt128(x num128) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.isZero():
		x.exp >>= 1 // sqrt(-0) is -0
		return x
	case x.neg:
		return c.invalid128()
	case x.kind == infinite:
		return x
	}
	k := 2*prec128 + 2 - x.coef.ndigits()
	if (int(x.exp)-k)&1 != 0 {
		k--
	}
	n := u256(x.coef).mulPow10(k)
	s := isqrt256(n)
	exact := n == s.mul(s)
	exp := (int(x.exp) - k) / 2
	if exact {
		// Move back toward the preferred exponent, floor(x.exp / 2).
		var z int
		s, z = trailingZeros128(s, int(x.exp>>1)-exp)
		exp += z
	}
	return c.round128(false, s, exp, !exact)
}
