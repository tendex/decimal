package decimal

import "math/bits"

// zeroSign returns the sign of an exactly zero sum of operands with
// differing signs: positive, except when rounding toward negative infinity.
func (c *Context) zeroSign() bool { return c.Rounding == ToNegativeInf }

// add returns x + y.
func (c *Context) add(f *format, x, y num) num {
	if x.kind|y.kind != finite {
		return c.addSpecial(f, x, y)
	}
	// Let x be the operand with the larger exponent. The preferred exponent
	// of the sum is the smaller one, y.exp.
	if x.exp < y.exp {
		x, y = y, x
	}
	if x.coef == 0 {
		// 0 + y is y exactly, at the preferred exponent.
		if y.coef == 0 && x.neg != y.neg {
			y.neg = c.zeroSign()
		}
		return y
	}
	gap := int(x.exp - y.exp)
	nx := ndigits64(x.coef)

	if gap+nx <= 19 {
		// x can be aligned to y.exp within 64 bits; the sum is exact.
		xc := x.coef * pow10tab[gap]
		var sum uint64
		neg := x.neg
		switch {
		case x.neg == y.neg:
			sum = xc + y.coef
		case xc > y.coef:
			sum = xc - y.coef
		case xc < y.coef:
			sum, neg = y.coef-xc, y.neg
		default:
			neg = c.zeroSign()
		}
		return c.round(f, neg, uint128{0, sum}, int(y.exp), false)
	}

	// Align x as far toward y.exp as 38 digits allow. If that is not far
	// enough, x has 38 digits, y is at least 22 digits smaller, and shifting
	// y right with a sticky bit loses nothing that rounding could observe.
	s := min(gap, 38-nx)
	xc := pow10tab128[s].mul64(x.coef)
	exp := int(x.exp) - s
	yc, sticky := y.coef, false
	if gap -= s; gap > 0 {
		if gap > 19 {
			yc, sticky = 0, y.coef != 0
		} else {
			d := pow10tab[gap]
			yc, sticky = y.coef/d, y.coef%d != 0
		}
	}
	neg := x.neg
	var sum uint128
	switch {
	case x.neg == y.neg:
		sum = xc.add64(yc)
	case xc.hi != 0 || xc.lo > yc:
		sum = xc.sub64(yc)
		if sticky {
			// x - (yc + ε) = (x - yc - 1) + (1 - ε)
			sum = sum.sub64(1)
		}
	case xc.lo < yc:
		// Only reachable when fully aligned, so there is no sticky bit.
		sum, neg = uint128{0, yc - xc.lo}, y.neg
	default:
		neg = c.zeroSign()
	}
	return c.round(f, neg, sum, exp, sticky)
}

// sub returns x - y.
func (c *Context) sub(f *format, x, y num) num {
	if !y.isNaN() {
		y.neg = !y.neg
	}
	return c.add(f, x, y)
}

func (c *Context) addSpecial(f *format, x, y num) num {
	switch {
	case x.isNaN() || y.isNaN():
		return c.nan(f, x, y)
	case x.kind == infinite && y.kind == infinite && x.neg != y.neg:
		return c.invalid() // Inf - Inf
	case x.kind == infinite:
		return num{kind: infinite, neg: x.neg}
	}
	return num{kind: infinite, neg: y.neg}
}

// mul returns x × y.
func (c *Context) mul(f *format, x, y num) num {
	neg := x.neg != y.neg
	if x.kind|y.kind != finite {
		switch {
		case x.isNaN() || y.isNaN():
			return c.nan(f, x, y)
		case x.isZero() || y.isZero():
			return c.invalid() // 0 × Inf
		}
		return num{kind: infinite, neg: neg}
	}
	hi, lo := bits.Mul64(x.coef, y.coef)
	return c.round(f, neg, uint128{hi, lo}, int(x.exp)+int(y.exp), false)
}

// quo returns x / y.
func (c *Context) quo(f *format, x, y num) num {
	neg := x.neg != y.neg
	if x.kind|y.kind != finite {
		switch {
		case x.isNaN() || y.isNaN():
			return c.nan(f, x, y)
		case x.kind == infinite && y.kind == infinite:
			return c.invalid() // Inf / Inf
		case x.kind == infinite:
			return num{kind: infinite, neg: neg}
		}
		return num{exp: int32(f.emin), neg: neg} // finite / Inf
	}
	if y.coef == 0 {
		if x.coef == 0 {
			return c.invalid() // 0 / 0
		}
		c.Flags |= DivisionByZero
		return num{kind: infinite, neg: neg}
	}
	exp := int(x.exp) - int(y.exp) // preferred exponent
	if x.coef == 0 {
		return c.round(f, neg, uint128{}, exp, false)
	}
	// Scale x so that the integer quotient has prec+1 or prec+2 digits.
	// Rounding then always discards at least one digit, and a sticky bit
	// for the remainder is all it needs to be correct.
	k := f.prec + 1 + ndigits64(y.coef) - ndigits64(x.coef)
	q, r := pow10tab128[k].mul64(x.coef).quoRem64(y.coef)
	exp -= k
	if r == 0 {
		// Exact: move back toward the preferred exponent.
		var z int
		q.lo, z = trailingZeros64(q.lo, k)
		exp += z
	}
	return c.round(f, neg, q, exp, r != 0)
}
