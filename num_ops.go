package decimal

import "math/bits"

// cmpAbs compares the magnitudes of two finite numbers.
func cmpAbs(x, y num) int {
	switch {
	case x.coef == 0 || y.coef == 0:
		return cmpUint64(x.coef, y.coef)
	case x.adjusted() != y.adjusted():
		return cmpInt(x.adjusted(), y.adjusted())
	}
	// Same adjusted exponent: the exponents differ by less than the
	// precision, and aligning the shorter coefficient cannot overflow.
	if x.exp >= y.exp {
		return cmpUint64(x.coef*pow10tab[x.exp-y.exp], y.coef)
	}
	return cmpUint64(x.coef, y.coef*pow10tab[y.exp-x.exp])
}

func cmpUint64(x, y uint64) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

func cmpInt(x, y int) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// cmpNum compares two non-NaN numbers by value.
func cmpNum(x, y num) int {
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
		r = cmpInt(int(x.kind), int(y.kind)) // finite < infinite
	default:
		r = cmpAbs(x, y)
	}
	if x.neg {
		return -r
	}
	return r
}

// compare implements the IEEE 754 quiet (signal == false) and signaling
// comparisons.
func (c *Context) compare(x, y num, signal bool) Ordering {
	if x.isNaN() || y.isNaN() {
		if signal || x.kind == signalingNaN || y.kind == signalingNaN {
			c.Flags |= Invalid
		}
		return Unordered
	}
	return Ordering(cmpNum(x, y))
}

// cmpTotal implements the IEEE 754 totalOrder relation as a three-way
// comparison:
//
//	-NaN < -sNaN < -Inf < -finite < -0 < +0 < +finite < +Inf < +sNaN < +NaN
//
// with NaNs ordered by payload and equal finite numbers by exponent.
func cmpTotal(x, y num) int {
	if x.neg != y.neg {
		if x.neg {
			return -1
		}
		return 1
	}
	var r int
	switch {
	case x.isNaN() || y.isNaN():
		// Any NaN is above every number; quiet is above signaling.
		rank := func(n num) int {
			switch n.kind {
			case quietNaN:
				return 2
			case signalingNaN:
				return 1
			}
			return 0
		}
		if r = cmpInt(rank(x), rank(y)); r == 0 {
			r = cmpUint64(x.coef, y.coef)
		}
	case x.kind == infinite || y.kind == infinite:
		r = cmpInt(int(x.kind), int(y.kind))
	default:
		if r = cmpAbs(x, y); r == 0 {
			r = cmpInt(int(x.exp), int(y.exp))
		}
	}
	if x.neg {
		return -r
	}
	return r
}

// minMax implements minimum, maximum (number == false) and minimumNumber,
// maximumNumber (number == true) of IEEE 754-2019, on values or magnitudes.
// A single quiet NaN operand is propagated by the former and treated as
// missing data by the latter. Among equal numbers the choice follows the
// total order, which makes the operations commutative.
func (c *Context) minMax(f *format, x, y num, max, mag, number bool) num {
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
		return c.nan(f, x, y)
	}
	var r int
	if mag {
		ax, ay := x, y
		ax.neg, ay.neg = false, false
		r = cmpNum(ax, ay)
	}
	if r == 0 {
		r = cmpNum(x, y)
	}
	if r == 0 {
		r = cmpTotal(x, y)
	}
	if (r > 0) == max {
		return x
	}
	return y
}

// quantize returns x rounded to the exponent exp.
func (c *Context) quantize(f *format, x num, exp int) num {
	if exp < f.emin || exp > f.emax {
		return c.invalid()
	}
	if x.coef == 0 {
		x.exp = int32(exp)
		return x
	}
	if k := int(x.exp) - exp; k >= 0 {
		// Pad with zeros, if they fit.
		if k > f.prec-ndigits64(x.coef) {
			return c.invalid()
		}
		x.coef *= pow10tab[k]
		x.exp = int32(exp)
		return x
	}
	q, rem := shiftRight(uint128{0, x.coef}, exp-int(x.exp))
	if rem != remZero {
		c.Flags |= Inexact
		if roundsUp(c.Rounding, x.neg, q&1 != 0, rem) {
			if q++; q > f.maxCoef {
				return c.invalid()
			}
		}
	}
	x.coef, x.exp = q, int32(exp)
	return x
}

// quantizeTo returns x with the exponent of y.
func (c *Context) quantizeTo(f *format, x, y num) num {
	switch {
	case x.isNaN() || y.isNaN():
		return c.nan(f, x, y)
	case x.kind == infinite && y.kind == infinite:
		return x
	case x.kind == infinite || y.kind == infinite:
		return c.invalid()
	}
	return c.quantize(f, x, int(y.exp))
}

func sameQuantum(x, y num) bool {
	switch {
	case x.isNaN() || y.isNaN():
		return x.isNaN() && y.isNaN()
	case x.kind == infinite || y.kind == infinite:
		return x.kind == y.kind
	}
	return x.exp == y.exp
}

// roundToIntegral rounds x to an integer in the given direction, raising
// the inexact exception only if exact is set.
func (c *Context) roundToIntegral(f *format, x num, mode RoundingMode, exact bool) num {
	switch {
	case x.isNaN():
		return c.nan(f, x, x)
	case x.kind == infinite || x.exp >= 0:
		return x
	}
	c2 := Context{Rounding: mode}
	x = c2.quantize(f, x, 0)
	if exact {
		c.Flags |= c2.Flags
	}
	return x
}

// next returns the neighbour of x in the direction of positive (up == true)
// or negative infinity.
func (c *Context) next(f *format, x num, up bool) num {
	switch {
	case x.isNaN():
		return c.nan(f, x, x)
	case x.kind == infinite:
		if x.neg != up {
			return x
		}
		return num{coef: f.maxCoef, exp: int32(f.emax), neg: x.neg}
	case x.coef == 0:
		return num{coef: 1, exp: int32(f.emin), neg: !up}
	}
	// Give x as many digits as it can hold: its neighbours differ by one
	// unit in that last place.
	k := min(f.prec-ndigits64(x.coef), int(x.exp)-f.emin)
	x.coef *= pow10tab[k]
	x.exp -= int32(k)
	switch {
	case x.neg != up: // away from zero
		if x.coef++; x.coef > f.maxCoef {
			x.coef = (f.maxCoef + 1) / 10
			if x.exp++; int(x.exp) > f.emax {
				return num{kind: infinite, neg: x.neg}
			}
		}
	case x.coef == (f.maxCoef+1)/10 && int(x.exp) > f.emin:
		x.coef = f.maxCoef
		x.exp--
	default:
		x.coef--
	}
	return x
}

// logB returns the adjusted exponent of x as a decimal number.
func (c *Context) logB(f *format, x num) num {
	switch {
	case x.isNaN():
		return c.nan(f, x, x)
	case x.kind == infinite:
		return num{kind: infinite}
	case x.coef == 0:
		c.Flags |= DivisionByZero
		return num{kind: infinite, neg: true}
	}
	e := x.adjusted()
	if e < 0 {
		return num{coef: uint64(-e), neg: true}
	}
	return num{coef: uint64(e)}
}

// scaleB returns x × 10**n.
func (c *Context) scaleB(f *format, x num, n int) num {
	switch {
	case x.isNaN():
		return c.nan(f, x, x)
	case x.kind == infinite:
		return x
	}
	// Exponents beyond this bound overflow or underflow regardless.
	const limit = 1 << 20
	return c.round(f, x.neg, uint128{0, x.coef}, int(x.exp)+min(max(n, -limit), limit), false)
}

// reduce returns x with trailing zeros removed from its coefficient.
func (c *Context) reduce(f *format, x num) num {
	switch {
	case x.isNaN():
		return c.nan(f, x, x)
	case x.kind == infinite:
		return x
	case x.coef == 0:
		x.exp = 0
		return x
	}
	coef, z := trailingZeros64(x.coef, f.prec)
	return c.round(f, x.neg, uint128{0, coef}, int(x.exp)+z, false)
}

// rem returns the remainder of x / y: x - y×n where n is the integer nearest
// the exact quotient, ties to even (nearest == true), as IEEE 754 defines
// remainder; or the quotient truncated toward zero, as for math.Mod. Both
// are always exact.
func (c *Context) rem(f *format, x, y num, nearest bool) num {
	switch {
	case x.isNaN() || y.isNaN():
		return c.nan(f, x, y)
	case x.kind == infinite || y.isZero():
		return c.invalid()
	case y.kind == infinite:
		return x
	}
	var r uint64 // x mod y, scaled by the smaller exponent
	var odd bool // parity of the truncated quotient
	var yc uint64
	exp := min(x.exp, y.exp)
	if x.exp >= y.exp {
		// Reduce x.coef × 10**gap modulo y.coef, up to 19 digits at a time.
		// Only the last partial quotient decides the parity.
		yc = y.coef
		q := x.coef / yc
		r = x.coef % yc
		for gap := int(x.exp - y.exp); gap > 0; {
			if r == 0 {
				q = 0 // the rest of the quotient is zeros: it is even
				break
			}
			k := min(gap, 19)
			hi, lo := bits.Mul64(r, pow10tab[k])
			q, r = bits.Div64(hi, lo, yc)
			gap -= k
		}
		odd = q&1 != 0
	} else {
		gap := int(y.exp - x.exp)
		if gap+ndigits64(y.coef) > f.prec+1 {
			// y is more than twice x: the quotient rounds to zero.
			return x
		}
		yc = y.coef * pow10tab[gap]
		r = x.coef % yc
		odd = x.coef/yc&1 != 0
	}
	neg := x.neg
	if nearest && (r > yc-r || r == yc-r && odd) {
		r = yc - r
		neg = !neg
	}
	if r == 0 {
		neg = x.neg
	}
	return c.round(f, neg, uint128{0, r}, int(exp), false)
}
