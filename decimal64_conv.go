package decimal

import "math"

// New64 returns coef × 10**exp, rounded to nearest even if coef has more
// than 16 digits. New64(1999, -2) is 19.99.
func New64(coef int64, exp int) Decimal64 {
	var c Context
	return c.New64(coef, exp)
}

// New64FromUint is like New64 for an unsigned coefficient.
func New64FromUint(coef uint64, exp int) Decimal64 {
	var c Context
	return c.New64FromUint(coef, exp)
}

// New64FromFloat returns the Decimal64 nearest to f: the exact binary value
// of f, rounded once, to nearest even. Values such as 2.5 that convert
// exactly get the exponent nearest zero. All others have as many digits as
// the format holds, not the few that would identify f: the float64 0.1 is
// exactly 0.1000000000000000055511151231257827..., and converts to that
// value rounded to 16 digits. To convert the way strconv prints,
// parse the output of strconv.FormatFloat instead.
func New64FromFloat(f float64) Decimal64 {
	var c Context
	return c.New64FromFloat(f)
}

// New64 returns coef × 10**exp, rounded according to the context if coef has
// more than 16 digits.
func (c *Context) New64(coef int64, exp int) Decimal64 {
	neg, mag := abs64(coef)
	return pack64(c.round(&format64, neg, uint128{0, mag}, clampExp(exp), false))
}

// New64FromUint is like New64 for an unsigned coefficient.
func (c *Context) New64FromUint(coef uint64, exp int) Decimal64 {
	return pack64(c.round(&format64, false, uint128{0, coef}, clampExp(exp), false))
}

// New64FromFloat returns f converted to a Decimal64, correctly rounded
// according to the context. A NaN converts to a quiet NaN.
func (c *Context) New64FromFloat(f float64) Decimal64 {
	switch {
	case math.IsNaN(f):
		return pack64(num{kind: quietNaN, neg: math.Signbit(f)})
	case math.IsInf(f, 0):
		return pack64(num{kind: infinite, neg: f < 0})
	}
	neg, coef, exp, sticky := fromFloat(f)
	return pack64(c.round(&format64, neg, coef, exp, sticky))
}

// Int64 returns x truncated toward zero to an int64, and whether that
// integer equals x. Values out of range, infinities included, convert to the
// nearest int64, and NaNs to zero; neither is exact.
func (x Decimal64) Int64() (v int64, exact bool) {
	n := x.unpack()
	return truncInt64(n.kind, n.neg, uint128{0, n.coef}, int(n.exp))
}

// Uint64 returns x truncated toward zero to a uint64, and whether that
// integer equals x. Values out of range, infinities included, convert to the
// nearest uint64, and NaNs to zero; neither is exact.
func (x Decimal64) Uint64() (v uint64, exact bool) {
	n := x.unpack()
	return truncUint64(n.kind, n.neg, uint128{0, n.coef}, int(n.exp))
}

// Float64 returns the float64 nearest to x, ties to even. Values too large
// for a float64 convert to an infinity.
func (x Decimal64) Float64() float64 {
	n := x.unpack()
	return toFloat(n.kind, n.neg, uint128{0, n.coef}, int(n.exp))
}

// Int64From64 returns x rounded to an integer according to the context. It
// raises Inexact if x is not an integer; callers wanting the plain
// convertToInteger operations of IEEE 754 rather than the convertToInteger
// Exact ones can ignore that. If x is a NaN, an infinity, or rounds to a
// value out of range, it raises Invalid and returns the nearest int64 (zero
// for a NaN).
func (c *Context) Int64From64(x Decimal64) int64 {
	n := x.unpack()
	return c.toInt64(n.kind, n.neg, uint128{0, n.coef}, int(n.exp))
}

// Uint64From64 is like Int64From64 for uint64.
func (c *Context) Uint64From64(x Decimal64) uint64 {
	n := x.unpack()
	return c.toUint64(n.kind, n.neg, uint128{0, n.coef}, int(n.exp))
}

// Float64From64 returns the float64 nearest to x, ties to even, raising
// Inexact, Overflow and Underflow as the conversion warrants and Invalid for
// a signaling NaN. The binary rounding direction is not affected by the
// context.
func (c *Context) Float64From64(x Decimal64) float64 {
	n := x.unpack()
	return c.toFloatFlags(n.kind, n.neg, uint128{0, n.coef}, int(n.exp))
}
