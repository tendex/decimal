package decimal

import "math/bits"

// wide is an unrounded intermediate result, (-1)**neg × coef × 10**exp. If
// sticky is set the exact value is some (coef + ε) × 10**exp with 0 < ε < 1,
// and coef is non-zero.
type wide struct {
	coef   uint256
	exp    int
	neg    bool
	sticky bool
}

// addWide returns x + y for exact operands of at most 68 digits, the size of
// a product of two decimal128 coefficients. The result is exact unless the
// operands are so far apart that the smaller cannot affect more than the
// rounding of the larger, in which case it is reduced to a sticky bit below
// at least 76 significant digits. The exponent of an exact result is the
// smaller of the operands' exponents whenever 77 digits allow it.
//
// It serves Decimal128 addition and fused multiply-add in all formats.
func (c *Context) addWide(x, y wide) wide {
	if x.exp < y.exp {
		x, y = y, x
	}
	if x.coef.isZero() {
		if y.coef.isZero() && x.neg != y.neg {
			y.neg = c.zeroSign()
		}
		return y
	}
	// Align x toward y.exp as far as 77 digits allow, then shift y right by
	// whatever gap remains.
	gap := x.exp - y.exp
	s := min(gap, 77-x.coef.ndigits())
	x.coef = x.coef.mulPow10(s)
	x.exp -= s
	if gap -= s; gap > 0 {
		var rem remainder
		y.coef, rem = y.coef.shiftRight(gap)
		x.sticky = rem != remZero
	}
	switch {
	case x.neg == y.neg:
		x.coef = x.coef.add(y.coef)
	case x.coef.cmp(y.coef) > 0:
		x.coef = x.coef.sub(y.coef)
		if x.sticky {
			// x - (y + ε) = (x - y - 1) + (1 - ε)
			x.coef = x.coef.sub64(1)
		}
	case x.coef.cmp(y.coef) < 0:
		// Only reachable when fully aligned, so there is no sticky bit.
		x.coef, x.neg = y.coef.sub(x.coef), y.neg
	default:
		x.coef, x.neg = uint256{}, c.zeroSign()
	}
	return x
}

// roundWide rounds w to format f.
func (c *Context) roundWide(f *format, w wide) num {
	if !w.coef.fits128() {
		// Truncate to 38 digits first, which leaves ample guard digits
		// above the 16 or fewer the format keeps.
		drop := w.coef.ndigits() - 38
		q, rem := w.coef.shiftRight(drop)
		w.coef, w.exp = q, w.exp+drop
		w.sticky = w.sticky || rem != remZero
	}
	return c.round(f, w.neg, w.coef.low128(), w.exp, w.sticky)
}

// nan3 is nan for three operands.
func (c *Context) nan3(f *format, x, y, z num) num {
	for _, n := range [...]num{x, y, z} {
		if n.kind == signalingNaN {
			return c.nan(f, n, n)
		}
	}
	for _, n := range [...]num{x, y, z} {
		if n.kind == quietNaN {
			return c.nan(f, n, n)
		}
	}
	panic("decimal: nan3 called without a NaN")
}

// fma returns x × y + z, rounded once.
func (c *Context) fma(f *format, x, y, z num) num {
	neg := x.neg != y.neg
	if x.kind|y.kind|z.kind != finite {
		switch {
		case x.isNaN() || y.isNaN():
			return c.nan3(f, x, y, z)
		case x.isZero() && y.kind == infinite, x.kind == infinite && y.isZero():
			// 0 × Inf is invalid whatever z is. IEEE 754 leaves the case
			// of a quiet NaN z to the implementation; like most hardware,
			// this one treats the multiplication as having happened.
			return c.invalid()
		case z.isNaN():
			return c.nan3(f, x, y, z)
		case x.kind == infinite || y.kind == infinite:
			if z.kind == infinite && z.neg != neg {
				return c.invalid() // Inf - Inf
			}
			return num{kind: infinite, neg: neg}
		}
		return z // finite × finite + Inf
	}
	hi, lo := bits.Mul64(x.coef, y.coef)
	p := wide{coef: uint256{lo, hi}, exp: int(x.exp) + int(y.exp), neg: neg}
	return c.roundWide(f, c.addWide(p, wide{coef: uint256{z.coef}, exp: int(z.exp), neg: z.neg}))
}
