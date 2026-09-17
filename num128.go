package decimal

// This file and its siblings hold the 128-bit kernel behind Decimal128. It
// mirrors the 64-bit kernel in num.go function for function (the names
// carry a 128 suffix), with uint128 coefficients and uint256 intermediates.
// The format is fixed, so there is no format descriptor to pass around.

const (
	prec128 = 34
	emax128 = 6111  // largest exponent q of a finite number
	emin128 = -6176 // smallest exponent q, that of the subnormals (Etiny)
)

var (
	maxCoef128    = pow10tab128[34].sub64(1) // 10**34 - 1
	maxPayload128 = pow10tab128[33].sub64(1) // 10**33 - 1
)

// num128 is an unpacked decimal128 value:
// (-1)**neg × coef × 10**exp when finite. For NaNs coef is the payload.
type num128 struct {
	coef uint128
	exp  int32
	neg  bool
	kind kind
}

func (n num128) isNaN() bool { return n.kind >= quietNaN }

func (n num128) isZero() bool { return n.kind == finite && n.coef.isZero() }

// adjusted returns the exponent of the most significant digit of a finite,
// non-zero n.
func (n num128) adjusted() int { return int(n.exp) + n.coef.ndigits() - 1 }

func (n num128) isSubnormal() bool {
	return n.kind == finite && !n.coef.isZero() && n.adjusted() < emin128+prec128-1
}

func (n num128) isNormal() bool {
	return n.kind == finite && !n.coef.isZero() && n.adjusted() >= emin128+prec128-1
}

func (n num128) isInteger() bool {
	switch {
	case n.kind != finite:
		return false
	case n.coef.isZero() || n.exp >= 0:
		return true
	case n.exp < -prec128:
		return false
	}
	_, r := n.coef.quoRem(pow10tab128[-n.exp])
	return r.isZero()
}

func (n num128) sign() int {
	switch {
	case n.isNaN() || n.isZero():
		return 0
	case n.neg:
		return -1
	}
	return 1
}

func (n num128) class() Class {
	var c Class
	switch {
	case n.kind == signalingNaN:
		return SignalingNaN
	case n.kind == quietNaN:
		return QuietNaN
	case n.kind == infinite:
		c = PositiveInf
	case n.coef.isZero():
		c = PositiveZero
	case n.isSubnormal():
		c = PositiveSubnormal
	default:
		c = PositiveNormal
	}
	if n.neg {
		c = NegativeInf + PositiveInf - c
	}
	return c
}

// invalid128 raises the invalid-operation exception and returns the default
// quiet NaN.
func (c *Context) invalid128() num128 {
	c.Flags |= Invalid
	return num128{kind: quietNaN}
}

// nan128 returns the NaN result of an operation with at least one NaN
// operand; see nan.
func (c *Context) nan128(x, y num128) num128 {
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
	if maxPayload128.less(r.coef) {
		r.coef = uint128{}
	}
	r.exp = 0
	return r
}

// nan3128 is nan128 for three operands.
func (c *Context) nan3128(x, y, z num128) num128 {
	for _, n := range [...]num128{x, y, z} {
		if n.kind == signalingNaN {
			return c.nan128(n, n)
		}
	}
	for _, n := range [...]num128{x, y, z} {
		if n.kind == quietNaN {
			return c.nan128(n, n)
		}
	}
	panic("decimal: nan3128 called without a NaN")
}

// round128 returns (-1)**neg × coef × 10**exp rounded to decimal128; see
// round.
func (c *Context) round128(neg bool, coef uint128, exp int, sticky bool) num128 {
	if !sticky && emin128 <= exp && exp <= emax128 && !maxCoef128.less(coef) {
		return num128{coef: coef, exp: int32(exp), neg: neg}
	}
	return c.roundWide128(wide{coef: u256(coef), exp: exp, neg: neg, sticky: sticky})
}

// roundWide128 rounds a 256-bit intermediate to decimal128.
func (c *Context) roundWide128(w wide) num128 {
	if w.coef.isZero() {
		return num128{exp: int32(min(max(w.exp, emin128), emax128)), neg: w.neg}
	}
	nd := w.coef.ndigits()
	// IEEE 754 detects tininess before rounding for decimal formats.
	tiny := w.exp+nd < emin128+prec128

	exp := w.exp
	drop := max(nd-prec128, emin128-exp)
	q := w.coef.low128()
	var rem remainder
	if drop > 0 {
		q, rem = shiftRight256(w.coef, drop)
		exp += drop
	}
	if w.sticky {
		rem = rem.sticky()
	}
	if rem != remZero {
		c.Flags |= Inexact
		if tiny {
			c.Flags |= Underflow
		}
		if roundsUp(c.Rounding, w.neg, q.lo&1 != 0, rem) {
			if q = q.add64(1); maxCoef128.less(q) {
				q = pow10tab128[prec128-1]
				exp++
			}
		}
	}
	if exp > emax128 {
		// Still representable if the coefficient has room for the excess
		// exponent as trailing zeros.
		k := exp - emax128
		if !q.isZero() && k > prec128-q.ndigits() {
			return c.overflow128(w.neg)
		}
		if !q.isZero() {
			q = q.mul(pow10tab128[k]).low128()
		}
		exp = emax128
	}
	return num128{coef: q, exp: int32(exp), neg: w.neg}
}

// overflow128 raises the overflow and inexact exceptions and returns the
// result the rounding direction calls for; see overflow.
func (c *Context) overflow128(neg bool) num128 {
	c.Flags |= Overflow | Inexact
	switch {
	case c.Rounding == ToZero,
		c.Rounding == ToPositiveInf && neg,
		c.Rounding == ToNegativeInf && !neg:
		return num128{coef: maxCoef128, exp: emax128, neg: neg}
	}
	return num128{kind: infinite, neg: neg}
}

// trailingZeros128 returns the largest n <= max such that 10**n divides x,
// together with x / 10**n. x must be non-zero.
func trailingZeros128(x uint128, max int) (uint128, int) {
	n := 0
	for _, s := range [...]int{16, 16, 8, 4, 2, 1} {
		if x.hi == 0 {
			break
		}
		if n+s > max {
			continue
		}
		if q, r := x.quoRem64(pow10tab[s]); r == 0 {
			x, n = q, n+s
		}
	}
	if x.hi == 0 {
		lo, z := trailingZeros64(x.lo, max-n)
		return uint128{0, lo}, n + z
	}
	return x, n
}

// parse128 converts s to decimal128; see parse.
func (c *Context) parse128(fn, s string) (num128, error) {
	r, ok := scan(s)
	if ok && r.kind >= quietNaN && maxPayload128.less(r.coef) {
		ok = false
	}
	if !ok {
		return num128{kind: quietNaN}, syntaxError(fn, s)
	}
	if r.kind != finite {
		return num128{coef: r.coef, neg: r.neg, kind: r.kind}, nil
	}
	return c.round128(r.neg, r.coef, r.exp, r.sticky), nil
}
