package decimal

// This file implements the IEEE 754 convertFormat operation between the
// three decimal formats. Widening conversions are exact; narrowing ones
// round. Either way a signaling NaN is quieted and raises Invalid, and a NaN
// payload too large for the destination is dropped.

func widen(n num) num128 {
	return num128{coef: uint128{0, n.coef}, exp: n.exp, neg: n.neg, kind: n.kind}
}

// narrow converts to the 64-bit kernel in format f.
func (c *Context) narrow(f *format, n num128) num {
	switch n.kind {
	case finite:
		return c.round(f, n.neg, n.coef, int(n.exp), false)
	case infinite:
		return num{kind: infinite, neg: n.neg}
	}
	r := num{coef: n.coef.lo, neg: n.neg, kind: n.kind}
	if n.coef.hi != 0 {
		r.coef = 0
	}
	return c.nan(f, r, r)
}

// convert converts between the two formats of the 64-bit kernel.
func (c *Context) convert(f *format, n num) num {
	switch n.kind {
	case finite:
		return c.round(f, n.neg, uint128{0, n.coef}, int(n.exp), false)
	case infinite:
		return n
	}
	return c.nan(f, n, n)
}

// Decimal64From32 returns x converted to a Decimal64, which is exact.
func (c *Context) Decimal64From32(x Decimal32) Decimal64 {
	return pack64(c.convert(&format64, x.unpack()))
}

// Decimal128From32 returns x converted to a Decimal128, which is exact.
func (c *Context) Decimal128From32(x Decimal32) Decimal128 {
	n := widen(x.unpack())
	if n.isNaN() {
		n = c.nan128(n, n)
	}
	return pack128(n)
}

// Decimal128From64 returns x converted to a Decimal128, which is exact.
func (c *Context) Decimal128From64(x Decimal64) Decimal128 {
	n := widen(x.unpack())
	if n.isNaN() {
		n = c.nan128(n, n)
	}
	return pack128(n)
}

// Decimal32From64 returns x rounded to a Decimal32.
func (c *Context) Decimal32From64(x Decimal64) Decimal32 {
	return pack32(c.convert(&format32, x.unpack()))
}

// Decimal32From128 returns x rounded to a Decimal32.
func (c *Context) Decimal32From128(x Decimal128) Decimal32 {
	return pack32(c.narrow(&format32, x.unpack()))
}

// Decimal64From128 returns x rounded to a Decimal64.
func (c *Context) Decimal64From128(x Decimal128) Decimal64 {
	return pack64(c.narrow(&format64, x.unpack()))
}

// Decimal64 returns x converted to a Decimal64, which is exact.
func (x Decimal32) Decimal64() Decimal64 {
	var c Context
	return c.Decimal64From32(x)
}

// Decimal128 returns x converted to a Decimal128, which is exact.
func (x Decimal32) Decimal128() Decimal128 {
	var c Context
	return c.Decimal128From32(x)
}

// Decimal32 returns x rounded, to nearest even, to a Decimal32.
func (x Decimal64) Decimal32() Decimal32 {
	var c Context
	return c.Decimal32From64(x)
}

// Decimal128 returns x converted to a Decimal128, which is exact.
func (x Decimal64) Decimal128() Decimal128 {
	var c Context
	return c.Decimal128From64(x)
}

// Decimal32 returns x rounded, to nearest even, to a Decimal32.
func (x Decimal128) Decimal32() Decimal32 {
	var c Context
	return c.Decimal32From128(x)
}

// Decimal64 returns x rounded, to nearest even, to a Decimal64.
func (x Decimal128) Decimal64() Decimal64 {
	var c Context
	return c.Decimal64From128(x)
}
