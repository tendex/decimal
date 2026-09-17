package decimal

// This file holds the 64-bit kernel shared by Decimal32 and Decimal64. Values
// are unpacked into a num, operated on with 64- and 128-bit integer
// arithmetic, and packed again, in the manner of runtime/softfloat64.go.
// Decimal128 has its own kernel on wider integers in num128.go.

// kind distinguishes finite numbers from the special values.
type kind uint8

const (
	finite kind = iota
	infinite
	quietNaN
	signalingNaN
)

// num is an unpacked decimal32 or decimal64 value:
// (-1)**neg × coef × 10**exp when finite. For NaNs coef is the payload.
type num struct {
	coef uint64
	exp  int32
	neg  bool
	kind kind
}

// format describes an interchange format to the 64-bit kernel.
type format struct {
	prec       int    // precision p in digits
	emax       int    // largest exponent q of a finite number
	emin       int    // smallest exponent q, that of the subnormals (Etiny)
	maxCoef    uint64 // 10**prec - 1
	maxPayload uint64 // largest canonical NaN payload
}

func (n num) isNaN() bool { return n.kind >= quietNaN }

func (n num) isZero() bool { return n.kind == finite && n.coef == 0 }

// adjusted returns the exponent of the most significant digit of a finite,
// non-zero n.
func (n num) adjusted() int { return int(n.exp) + ndigits64(n.coef) - 1 }

func (n num) isSubnormal(f *format) bool {
	return n.kind == finite && n.coef != 0 && n.adjusted() < f.emin+f.prec-1
}

func (n num) isNormal(f *format) bool {
	return n.kind == finite && n.coef != 0 && n.adjusted() >= f.emin+f.prec-1
}

func (n num) isInteger() bool {
	switch {
	case n.kind != finite:
		return false
	case n.coef == 0 || n.exp >= 0:
		return true
	case n.exp <= -20:
		return false
	}
	return n.coef%pow10tab[-n.exp] == 0
}

func (n num) sign() int {
	switch {
	case n.isNaN() || n.isZero():
		return 0
	case n.neg:
		return -1
	}
	return 1
}

func (n num) class(f *format) Class {
	var c Class
	switch {
	case n.kind == signalingNaN:
		return SignalingNaN
	case n.kind == quietNaN:
		return QuietNaN
	case n.kind == infinite:
		c = PositiveInf
	case n.coef == 0:
		c = PositiveZero
	case n.isSubnormal(f):
		c = PositiveSubnormal
	default:
		c = PositiveNormal
	}
	if n.neg {
		c = NegativeInf + PositiveInf - c
	}
	return c
}

// invalid raises the invalid-operation exception and returns the default
// quiet NaN.
func (c *Context) invalid() num {
	c.Flags |= Invalid
	return num{kind: quietNaN}
}

// nan returns the NaN result of an operation with at least one NaN operand:
// the first signaling NaN, quieted, or failing that the first quiet NaN.
// Signaling NaNs raise the invalid-operation exception.
func (c *Context) nan(f *format, x, y num) num {
	r := x
	switch {
	case x.kind == signalingNaN:
	case y.kind == signalingNaN || x.kind != quietNaN:
		r = y
	}
	if r.kind == signalingNaN {
		c.Flags |= Invalid
		r.kind = quietNaN
	}
	if r.coef > f.maxPayload {
		r.coef = 0
	}
	r.exp = 0
	return r
}

// A remainder classifies the digits discarded by a rounding step relative to
// one unit in the last retained place.
type remainder uint8

const (
	remZero  remainder = iota // nothing but zeros discarded
	remBelow                  // less than half
	remHalf                   // exactly half
	remAbove                  // more than half
)

func classify(r, half uint64) remainder {
	switch {
	case r == 0:
		return remZero
	case r < half:
		return remBelow
	case r == half:
		return remHalf
	}
	return remAbove
}

// sticky folds additional non-zero digits beyond those already classified
// into rem.
func (rem remainder) sticky() remainder { return rem | remBelow }

// roundsUp reports whether a magnitude whose discarded digits are rem, and
// which is therefore inexact, is rounded away from zero.
func roundsUp(mode RoundingMode, neg, odd bool, rem remainder) bool {
	switch mode {
	case ToNearestEven:
		return rem == remAbove || rem == remHalf && odd
	case ToNearestAway:
		return rem >= remHalf
	case ToPositiveInf:
		return !neg
	case ToNegativeInf:
		return neg
	}
	return false
}

// shiftRight returns coef / 10**drop, which the caller guarantees fits in 64
// bits, and a classification of the remainder. drop must be positive.
func shiftRight(coef uint128, drop int) (uint64, remainder) {
	switch {
	case coef.isZero():
		return 0, remZero
	case drop > 38:
		// coef < 2**128 < 5×10**38: everything goes, and it is below half.
		return 0, remBelow
	case drop <= 19:
		d := pow10tab[drop]
		q, r := coef.quoRem64(d)
		return q.lo, classify(r, d/2)
	}
	// 10**drop does not fit in 64 bits: divide in two steps and combine the
	// remainders, r = r2×10**19 + r1.
	q1, r1 := coef.quoRem64(1e19)
	d := pow10tab[drop-19]
	q2, r2 := q1.quoRem64(d)
	rem := classify(r2, d/2)
	if r1 != 0 {
		rem = rem.sticky()
	}
	return q2.lo, rem
}

// round returns (-1)**neg × coef × 10**exp rounded to format f, raising
// the inexact, underflow and overflow exceptions as required. If sticky is
// set the exact value is some (coef + ε) × 10**exp with 0 < ε < 1; coef must
// then be non-zero.
//
// An exact result keeps the given exponent when possible, and otherwise
// moves it by as little as possible, which is what the preferred-exponent
// rules of IEEE 754 require of every operation.
func (c *Context) round(f *format, neg bool, coef uint128, exp int, sticky bool) num {
	if coef.hi == 0 && coef.lo <= f.maxCoef && !sticky && f.emin <= exp && exp <= f.emax {
		return num{coef: coef.lo, exp: int32(exp), neg: neg}
	}
	return c.roundSlow(f, neg, coef, exp, sticky)
}

func (c *Context) roundSlow(f *format, neg bool, coef uint128, exp int, sticky bool) num {
	if coef.isZero() {
		return num{exp: int32(min(max(exp, f.emin), f.emax)), neg: neg}
	}
	nd := coef.ndigits()
	// IEEE 754 detects tininess before rounding for decimal formats.
	tiny := exp+nd < f.emin+f.prec

	drop := max(nd-f.prec, f.emin-exp)
	q := coef.lo
	var rem remainder
	if drop > 0 {
		q, rem = shiftRight(coef, drop)
		exp += drop
	}
	if sticky {
		rem = rem.sticky()
	}
	if rem != remZero {
		c.Flags |= Inexact
		if tiny {
			c.Flags |= Underflow
		}
		if roundsUp(c.Rounding, neg, q&1 != 0, rem) {
			if q++; q > f.maxCoef {
				q = (f.maxCoef + 1) / 10
				exp++
			}
		}
	}
	if exp > f.emax {
		// Still representable if the coefficient has room for the excess
		// exponent as trailing zeros.
		k := exp - f.emax
		if q != 0 && k > f.prec-ndigits64(q) {
			return c.overflow(f, neg)
		}
		if q != 0 {
			q *= pow10tab[k]
		}
		exp = f.emax
	}
	return num{coef: q, exp: int32(exp), neg: neg}
}

// overflow raises the overflow and inexact exceptions and returns the
// result the rounding direction calls for: infinity, or the largest finite
// number when rounding toward zero from it.
func (c *Context) overflow(f *format, neg bool) num {
	c.Flags |= Overflow | Inexact
	switch {
	case c.Rounding == ToZero,
		c.Rounding == ToPositiveInf && neg,
		c.Rounding == ToNegativeInf && !neg:
		return num{coef: f.maxCoef, exp: int32(f.emax), neg: neg}
	}
	return num{kind: infinite, neg: neg}
}
