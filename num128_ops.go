package decimal

import "cmp"

// cmpAbs128 compares the magnitudes of two finite numbers.
func cmpAbs128(x, y num128) int {
	switch {
	case x.coef.isZero() || y.coef.isZero() || x.exp == y.exp:
		return x.coef.cmp(y.coef)
	case x.adjusted() != y.adjusted():
		return cmp.Compare(x.adjusted(), y.adjusted())
	}
	// Same adjusted exponent: the exponents differ by less than the
	// precision, and aligning the shorter coefficient cannot overflow.
	if x.exp >= y.exp {
		return x.coef.scale(int(x.exp - y.exp)).cmp(y.coef)
	}
	return x.coef.cmp(y.coef.scale(int(y.exp - x.exp)))
}

// cmpNum128 compares two non-NaN numbers by value.
func cmpNum128(x, y num128) int {
	switch {
	case x.isZero() && y.isZero():
		return 0
	case x.neg != y.neg:
		if x.neg {
			return -1
		}
		return 1
	}
	var r int
	switch {
	case x.kind == infinite || y.kind == infinite:
		r = cmp.Compare(x.kind, y.kind) // finite < infinite
	default:
		r = cmpAbs128(x, y)
	}
	if x.neg {
		return -r
	}
	return r
}

// compare128 implements the quiet and signaling comparisons; see compare.
func (c *Context) compare128(x, y num128, signal bool) Ordering {
	if x.isNaN() || y.isNaN() {
		if signal || x.kind == signalingNaN || y.kind == signalingNaN {
			c.Flags |= Invalid
		}
		return Unordered
	}
	return Ordering(cmpNum128(x, y))
}

// cmpTotal128 implements the totalOrder relation; see cmpTotal.
func cmpTotal128(x, y num128) int {
	if x.neg != y.neg {
		if x.neg {
			return -1
		}
		return 1
	}
	var r int
	switch {
	case x.isNaN() || y.isNaN():
		rank := func(n num128) int {
			switch n.kind {
			case quietNaN:
				return 2
			case signalingNaN:
				return 1
			}
			return 0
		}
		if r = cmp.Compare(rank(x), rank(y)); r == 0 {
			r = x.coef.cmp(y.coef)
		}
	case x.kind == infinite || y.kind == infinite:
		r = cmp.Compare(x.kind, y.kind)
	default:
		if r = cmpAbs128(x, y); r == 0 {
			r = cmp.Compare(x.exp, y.exp)
		}
	}
	if x.neg {
		return -r
	}
	return r
}

// minMax128 implements the minimum and maximum operations; see minMax.
func (c *Context) minMax128(x, y num128, max, mag, number bool) num128 {
	if x.isNaN() || y.isNaN() {
		if number && !(x.isNaN() && y.isNaN()) {
			if x.kind == signalingNaN || y.kind == signalingNaN {
				c.Flags |= Invalid
			}
			if x.isNaN() {
				return y
			}
			return x
		}
		return c.nan128(x, y)
	}
	var r int
	if mag {
		ax, ay := x, y
		ax.neg, ay.neg = false, false
		r = cmpNum128(ax, ay)
	}
	if r == 0 {
		r = cmpNum128(x, y)
	}
	if r == 0 {
		r = cmpTotal128(x, y)
	}
	if (r > 0) == max {
		return x
	}
	return y
}

// quantize128 returns x rounded to the exponent exp.
func (c *Context) quantize128(x num128, exp int) num128 {
	if exp < emin128 || exp > emax128 {
		return c.invalid128()
	}
	if x.coef.isZero() {
		x.exp = int32(exp)
		return x
	}
	if k := int(x.exp) - exp; k >= 0 {
		// Pad with zeros, if they fit.
		if k > prec128-x.coef.ndigits() {
			return c.invalid128()
		}
		x.coef = x.coef.scale(k)
		x.exp = int32(exp)
		return x
	}
	q, rem := shiftRight256(u256(x.coef), exp-int(x.exp))
	if rem != remZero {
		c.Flags |= Inexact
		if roundsUp(c.Rounding, x.neg, q.lo&1 != 0, rem) {
			if q = q.add64(1); maxCoef128.less(q) {
				return c.invalid128()
			}
		}
	}
	x.coef, x.exp = q, int32(exp)
	return x
}

// quantizeTo128 returns x with the exponent of y.
func (c *Context) quantizeTo128(x, y num128) num128 {
	switch {
	case x.isNaN() || y.isNaN():
		return c.nan128(x, y)
	case x.kind == infinite && y.kind == infinite:
		return x
	case x.kind == infinite || y.kind == infinite:
		return c.invalid128()
	}
	return c.quantize128(x, int(y.exp))
}

// quantum128 returns 1 × 10**x.exp, one unit in the last place of x.
func (c *Context) quantum128(x num128) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.kind == infinite:
		return num128{kind: infinite}
	}
	return num128{coef: uint128{0, 1}, exp: x.exp}
}

func sameQuantum128(x, y num128) bool {
	switch {
	case x.isNaN() || y.isNaN():
		return x.isNaN() && y.isNaN()
	case x.kind == infinite || y.kind == infinite:
		return x.kind == y.kind
	}
	return x.exp == y.exp
}

// roundToIntegral128 rounds x to an integer; see roundToIntegral.
func (c *Context) roundToIntegral128(x num128, mode RoundingMode, exact bool) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.kind == infinite || x.exp >= 0:
		return x
	}
	c2 := Context{Rounding: mode}
	x = c2.quantize128(x, 0)
	if exact {
		c.Flags |= c2.Flags
	}
	return x
}

// next128 returns the neighbour of x in the given direction; see next.
func (c *Context) next128(x num128, up bool) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.kind == infinite:
		if x.neg != up {
			return x
		}
		return num128{coef: maxCoef128, exp: emax128, neg: x.neg}
	case x.coef.isZero():
		return num128{coef: uint128{0, 1}, exp: emin128, neg: !up}
	}
	k := min(prec128-x.coef.ndigits(), int(x.exp)-emin128)
	x.coef = x.coef.scale(k)
	x.exp -= int32(k)
	switch {
	case x.neg != up: // away from zero
		if x.coef = x.coef.add64(1); maxCoef128.less(x.coef) {
			x.coef = pow10tab128[prec128-1]
			if x.exp++; x.exp > emax128 {
				return num128{kind: infinite, neg: x.neg}
			}
		}
	case x.coef == pow10tab128[prec128-1] && x.exp > emin128:
		x.coef = maxCoef128
		x.exp--
	default:
		x.coef = x.coef.sub64(1)
	}
	return x
}

// logB128 returns the adjusted exponent of x as a decimal number.
func (c *Context) logB128(x num128) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.kind == infinite:
		return num128{kind: infinite}
	case x.coef.isZero():
		c.Flags |= DivisionByZero
		return num128{kind: infinite, neg: true}
	}
	e := x.adjusted()
	if e < 0 {
		return num128{coef: uint128{0, uint64(-e)}, neg: true}
	}
	return num128{coef: uint128{0, uint64(e)}}
}

// scaleB128 returns x × 10**n.
func (c *Context) scaleB128(x num128, n int) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.kind == infinite:
		return x
	}
	const limit = 1 << 20
	return c.round128(x.neg, x.coef, int(x.exp)+min(max(n, -limit), limit), false)
}

// reduce128 returns x with trailing zeros removed from its coefficient.
func (c *Context) reduce128(x num128) num128 {
	switch {
	case x.isNaN():
		return c.nan128(x, x)
	case x.kind == infinite:
		return x
	case x.coef.isZero():
		x.exp = 0
		return x
	}
	coef, z := trailingZeros128(x.coef, prec128)
	return c.round128(x.neg, coef, int(x.exp)+z, false)
}

// rem128 returns the remainder of x / y; see rem.
func (c *Context) rem128(x, y num128, nearest bool) num128 {
	switch {
	case x.isNaN() || y.isNaN():
		return c.nan128(x, y)
	case x.kind == infinite || y.isZero():
		return c.invalid128()
	case y.kind == infinite:
		return x
	}
	var r, yc uint128 // x mod y and y, scaled by the smaller exponent
	var odd bool      // parity of the truncated quotient
	exp := min(x.exp, y.exp)
	if x.exp >= y.exp {
		// Reduce x.coef × 10**gap modulo y.coef, up to 38 digits at a time.
		// Only the last partial quotient decides the parity.
		yc = y.coef
		q, rr := x.coef.quoRem(yc)
		r = rr
		for gap := int(x.exp - y.exp); gap > 0; {
			if r.isZero() {
				q = uint128{} // the rest of the quotient is zeros: it is even
				break
			}
			k := min(gap, 38)
			q256, rr := r.mul(pow10tab128[k]).quoRem128(yc)
			q, r = q256.low128(), rr
			gap -= k
		}
		odd = q.lo&1 != 0
	} else {
		gap := int(y.exp - x.exp)
		if gap+y.coef.ndigits() > prec128+1 {
			// y is more than twice x: the quotient rounds to zero.
			return x
		}
		yc = y.coef.scale(gap)
		var q uint128
		q, r = x.coef.quoRem(yc)
		odd = q.lo&1 != 0
	}
	neg := x.neg
	if half := yc.sub(r); nearest && (half.less(r) || half == r && odd) {
		r = half
		neg = !neg
	}
	if r.isZero() {
		neg = x.neg
	}
	return c.round128(neg, r, int(exp), false)
}

// classifyRem128 classifies the remainder r of a division by d, r < d,
// against half of d.
func classifyRem128(r, d uint128) remainder {
	if r.isZero() {
		return remZero
	}
	return remBelow + remainder(1+r.lsh(1).cmp(d))
}

// roundToMultiple128 returns x rounded to a multiple of y; see
// roundToMultiple.
func (c *Context) roundToMultiple128(x, y num128) num128 {
	switch {
	case x.isNaN() || y.isNaN():
		return c.nan128(x, y)
	case x.kind == infinite || y.kind == infinite || y.neg || y.coef.isZero():
		return c.invalid128()
	case x.coef.isZero():
		x.exp = y.exp
		return x
	}
	var t uint128
	var rem remainder
	gap := int(x.exp - y.exp)
	switch {
	case gap >= 0:
		if gap > prec128 {
			return c.invalid128()
		}
		q, r := u256(x.coef).mulPow10(gap).quoRem128(y.coef)
		if !q.fits128() || maxCoef128.less(q.low128()) {
			return c.invalid128()
		}
		t, rem = q.low128(), classifyRem128(r, y.coef)
	case -gap > prec128+1:
		rem = remBelow
	default:
		d := u256(y.coef).mulPow10(-gap)
		if !d.fits128() || x.coef.less(d.low128()) {
			rem = remBelow + remainder(1+u256(x.coef).lsh(1).cmp(d))
		} else {
			var r uint128
			t, r = x.coef.quoRem(d.low128())
			rem = classifyRem128(r, d.low128())
		}
	}
	var inc uint64
	if rem != remZero {
		inc = roundInc(c.Rounding, x.neg, t.lo&1, rem)
	}
	p := t.add64(inc).mul(y.coef)
	if !p.fits128() || maxCoef128.less(p.low128()) {
		return c.invalid128()
	}
	if rem != remZero {
		c.Flags |= Inexact
	}
	return num128{coef: p.low128(), exp: y.exp, neg: x.neg}
}
