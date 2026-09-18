package decimal

// Compare64 compares x and y by value and returns Less, Equal, Greater, or
// Unordered if either is a NaN. It is the IEEE 754 quiet comparison: only a
// signaling NaN raises Invalid. Members of the same cohort, and zeros of
// either sign, compare Equal.
func (c *Context) Compare64(x, y Decimal64) Ordering {
	return c.compare(x.unpack(), y.unpack(), false)
}

// CompareSignal64 is like Compare64 but raises Invalid for any NaN operand,
// as the IEEE 754 signaling comparisons do.
func (c *Context) CompareSignal64(x, y Decimal64) Ordering {
	return c.compare(x.unpack(), y.unpack(), true)
}

// Min64 returns the smaller of x and y, or a NaN if either is a NaN
// (IEEE 754-2019 minimum). -0 is smaller than +0.
func (c *Context) Min64(x, y Decimal64) Decimal64 {
	return pack64(c.minMax(&format64, x.unpack(), y.unpack(), false, false, false))
}

// Max64 returns the larger of x and y, or a NaN if either is a NaN
// (IEEE 754-2019 maximum). +0 is larger than -0.
func (c *Context) Max64(x, y Decimal64) Decimal64 {
	return pack64(c.minMax(&format64, x.unpack(), y.unpack(), true, false, false))
}

// MinNum64 is like Min64 but treats a NaN as missing data: if exactly one
// operand is a NaN the other is returned (IEEE 754-2019 minimumNumber).
func (c *Context) MinNum64(x, y Decimal64) Decimal64 {
	return pack64(c.minMax(&format64, x.unpack(), y.unpack(), false, false, true))
}

// MaxNum64 is like Max64 but treats a NaN as missing data: if exactly one
// operand is a NaN the other is returned (IEEE 754-2019 maximumNumber).
func (c *Context) MaxNum64(x, y Decimal64) Decimal64 {
	return pack64(c.minMax(&format64, x.unpack(), y.unpack(), true, false, true))
}

// Quantize64 returns x rounded to the exponent of y. It raises Invalid and
// returns a NaN if the result would need more than 16 digits, or if exactly
// one operand is infinite.
func (c *Context) Quantize64(x, y Decimal64) Decimal64 {
	return pack64(c.quantizeTo(&format64, x.unpack(), y.unpack()))
}

// RoundToMultiple64 returns x rounded, in the direction of the context, to a
// multiple of y, which must be finite and positive. The result has the
// exponent of y, as Quantize64 gives it: 10.12 rounded to a multiple of 0.05
// is 10.10, and to a multiple of 0.25 is 10.00. Quantize64 is the special
// case of y a power of ten. It raises Inexact if the result differs from x,
// and Invalid, returning a NaN, if x is infinite, if y is not a finite
// positive number, or if the result would need more than 16 digits.
func (c *Context) RoundToMultiple64(x, y Decimal64) Decimal64 {
	return pack64(c.roundToMultiple(&format64, x.unpack(), y.unpack()))
}

// Quantum64 returns the quantum of x, 1 × 10**exponent: the value of one
// unit in the last place of its coefficient. The quantum of an infinity is
// +Inf.
func (c *Context) Quantum64(x Decimal64) Decimal64 {
	return pack64(c.quantum(&format64, x.unpack()))
}

// Round64 returns x rounded to the given number of decimal places, which may
// be negative to round to tens, hundreds and so on. Round64 only ever
// removes digits: a value with fewer decimal places is returned unchanged.
func (c *Context) Round64(x Decimal64, places int) Decimal64 {
	n := x.unpack()
	switch {
	case n.isNaN():
		return pack64(c.nan(&format64, n, n))
	case n.kind == infinite || int(n.exp) >= -places:
		return x
	}
	return pack64(c.quantize(&format64, n, max(-places, format64.emin)))
}

// RoundToIntegralExact64 rounds x to an integer in the rounding direction of
// the context, raising Inexact if the result differs from x.
func (c *Context) RoundToIntegralExact64(x Decimal64) Decimal64 {
	return pack64(c.roundToIntegral(&format64, x.unpack(), c.Rounding, true))
}

// RoundToIntegral64 rounds x to an integer in the given direction. Unlike
// RoundToIntegralExact64 it does not raise Inexact.
func (c *Context) RoundToIntegral64(x Decimal64, mode RoundingMode) Decimal64 {
	return pack64(c.roundToIntegral(&format64, x.unpack(), mode, false))
}

// NextUp64 returns the least Decimal64 that compares greater than x.
func (c *Context) NextUp64(x Decimal64) Decimal64 {
	return pack64(c.next(&format64, x.unpack(), true))
}

// NextDown64 returns the greatest Decimal64 that compares less than x.
func (c *Context) NextDown64(x Decimal64) Decimal64 {
	return pack64(c.next(&format64, x.unpack(), false))
}

// LogB64 returns the exponent of x as though x were normalized to one digit
// before the decimal point: floor(log10(|x|)). LogB64(±Inf) is +Inf;
// LogB64(0) is -Inf and raises DivisionByZero.
func (c *Context) LogB64(x Decimal64) Decimal64 {
	return pack64(c.logB(&format64, x.unpack()))
}

// ScaleB64 returns x × 10**n.
func (c *Context) ScaleB64(x Decimal64, n int) Decimal64 {
	return pack64(c.scaleB(&format64, x.unpack(), n))
}

// Reduce64 returns x with all trailing zeros removed from its coefficient,
// the simplest member of its cohort. All zeros reduce to 0E+0.
func (c *Context) Reduce64(x Decimal64) Decimal64 {
	return pack64(c.reduce(&format64, x.unpack()))
}

// Remainder64 returns the IEEE 754 remainder of x / y: x - y×n, where n is
// the integer nearest the exact quotient x / y, ties to even. The result is
// always exact and lies in [-|y|/2, |y|/2]. Compare math.Remainder.
func (c *Context) Remainder64(x, y Decimal64) Decimal64 {
	return pack64(c.rem(&format64, x.unpack(), y.unpack(), true))
}

// Mod64 returns the remainder of x / y with the quotient truncated toward
// zero, so the result has the sign of x and magnitude less than |y|. The
// result is always exact. Compare math.Mod.
func (c *Context) Mod64(x, y Decimal64) Decimal64 {
	return pack64(c.rem(&format64, x.unpack(), y.unpack(), false))
}

// Compare compares x and y by value and returns Less, Equal, Greater, or
// Unordered if either is a NaN.
func (x Decimal64) Compare(y Decimal64) Ordering {
	var c Context
	return c.Compare64(x, y)
}

// Equal reports whether x and y are equal in value. Members of the same
// cohort are equal, as are +0 and -0; a NaN is not equal to anything,
// itself included.
func (x Decimal64) Equal(y Decimal64) bool { return x.Compare(y) == Equal }

// Less reports whether x < y. It is false if either operand is a NaN.
func (x Decimal64) Less(y Decimal64) bool { return x.Compare(y) == Less }

// Cmp compares x and y by value and returns -1, 0 or +1. Like cmp.Compare
// on floating-point numbers, it treats a NaN as less than any number and
// equal to any other NaN, so that it is a consistent ordering for sorting:
//
//	slices.SortFunc(s, decimal.Decimal64.Cmp)
func (x Decimal64) Cmp(y Decimal64) int {
	switch o := x.Compare(y); {
	case o != Unordered:
		return int(o)
	case !x.IsNaN():
		return 1
	case !y.IsNaN():
		return -1
	}
	return 0
}

// CmpTotal compares the representations of x and y using the IEEE 754
// totalOrder relation and returns -1, 0 or +1. Unlike Cmp it distinguishes
// every pair of distinct canonical values:
//
//	-NaN < -sNaN < -Inf < -1.0 < -1.00 < -0 < +0 < 1.00 < 1.0 < +Inf < +sNaN < +NaN
//
// The totalOrder predicate of IEEE 754 is x.CmpTotal(y) <= 0.
func (x Decimal64) CmpTotal(y Decimal64) int { return cmpTotal(x.unpack(), y.unpack()) }

// Min returns the smaller of x and y, or a NaN if either is a NaN.
func (x Decimal64) Min(y Decimal64) Decimal64 {
	var c Context
	return c.Min64(x, y)
}

// Max returns the larger of x and y, or a NaN if either is a NaN.
func (x Decimal64) Max(y Decimal64) Decimal64 {
	var c Context
	return c.Max64(x, y)
}

// Quantize returns x rounded, to nearest even, to the exponent of y. For
// example, quantizing to 0.01 rounds to two decimal places and pads with
// zeros as necessary. The result is a NaN if it would need more than 16
// digits.
func (x Decimal64) Quantize(y Decimal64) Decimal64 {
	var c Context
	return c.Quantize64(x, y)
}

// RoundToMultiple returns x rounded, to nearest even, to a multiple of y,
// such as a tick size or a coin: 10.12 to a multiple of 0.05 is 10.10. The
// result has the exponent of y. See Context.RoundToMultiple64.
func (x Decimal64) RoundToMultiple(y Decimal64) Decimal64 {
	var c Context
	return c.RoundToMultiple64(x, y)
}

// Quantum returns the quantum of x, 1 × 10**exponent; the quantum of 12.50
// is 0.01.
func (x Decimal64) Quantum() Decimal64 {
	var c Context
	return c.Quantum64(x)
}

// SameQuantum reports whether x and y have the same exponent. Two NaNs, or
// two infinities, also have the same quantum.
func (x Decimal64) SameQuantum(y Decimal64) bool { return sameQuantum(x.unpack(), y.unpack()) }

// Round returns x rounded, to nearest even, to the given number of decimal
// places; see Context.Round64.
func (x Decimal64) Round(places int) Decimal64 {
	var c Context
	return c.Round64(x, places)
}

// RoundToIntegral returns x rounded to an integer in the given direction.
func (x Decimal64) RoundToIntegral(mode RoundingMode) Decimal64 {
	var c Context
	return c.RoundToIntegral64(x, mode)
}

// NextUp returns the least Decimal64 that compares greater than x.
func (x Decimal64) NextUp() Decimal64 {
	var c Context
	return c.NextUp64(x)
}

// NextDown returns the greatest Decimal64 that compares less than x.
func (x Decimal64) NextDown() Decimal64 {
	var c Context
	return c.NextDown64(x)
}

// LogB returns floor(log10(|x|)), the exponent of the most significant
// digit of x; see Context.LogB64.
func (x Decimal64) LogB() Decimal64 {
	var c Context
	return c.LogB64(x)
}

// ScaleB returns x × 10**n, rounded to nearest even if it is subnormal.
func (x Decimal64) ScaleB(n int) Decimal64 {
	var c Context
	return c.ScaleB64(x, n)
}

// Reduce returns x with all trailing zeros removed from its coefficient.
// Equal non-zero values reduce to identical representations, which makes
// x.Reduce().Bits() usable as a map key (zeros keep their sign).
func (x Decimal64) Reduce() Decimal64 {
	var c Context
	return c.Reduce64(x)
}

// Remainder returns the IEEE 754 remainder of x / y; see
// Context.Remainder64.
func (x Decimal64) Remainder(y Decimal64) Decimal64 {
	var c Context
	return c.Remainder64(x, y)
}

// Mod returns the remainder of x / y with the sign of x; see Context.Mod64.
func (x Decimal64) Mod(y Decimal64) Decimal64 {
	var c Context
	return c.Mod64(x, y)
}
