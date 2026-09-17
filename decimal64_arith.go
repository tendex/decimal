package decimal

// Add64 returns the sum x + y.
func (c *Context) Add64(x, y Decimal64) Decimal64 {
	return pack64(c.add(&format64, x.unpack(), y.unpack()))
}

// Sub64 returns the difference x - y.
func (c *Context) Sub64(x, y Decimal64) Decimal64 {
	return pack64(c.sub(&format64, x.unpack(), y.unpack()))
}

// Mul64 returns the product x × y.
func (c *Context) Mul64(x, y Decimal64) Decimal64 {
	return pack64(c.mul(&format64, x.unpack(), y.unpack()))
}

// Quo64 returns the quotient x / y.
func (c *Context) Quo64(x, y Decimal64) Decimal64 {
	return pack64(c.quo(&format64, x.unpack(), y.unpack()))
}

// FMA64 returns the fused multiply-add x × y + z: the exact product is added
// to z and the sum rounded once.
func (c *Context) FMA64(x, y, z Decimal64) Decimal64 {
	return pack64(c.fma(&format64, x.unpack(), y.unpack(), z.unpack()))
}

// Sqrt64 returns the square root of x, correctly rounded. The square root
// of a negative number other than -0 is a NaN and raises Invalid.
func (c *Context) Sqrt64(x Decimal64) Decimal64 {
	return pack64(c.sqrt(&format64, x.unpack()))
}

// FMA returns the fused multiply-add x × y + z, rounded once, to nearest
// even.
func (x Decimal64) FMA(y, z Decimal64) Decimal64 {
	var c Context
	return c.FMA64(x, y, z)
}

// Sqrt returns the square root of x, rounded to nearest even.
//
// The exponent of an exact square root is as close to half the exponent of
// x as the result allows, so the square root of 4.00 is 2.0.
func (x Decimal64) Sqrt() Decimal64 {
	var c Context
	return c.Sqrt64(x)
}

// Add returns the sum x + y, rounded to nearest even.
//
// The exponent of an exact sum is the smaller of the operands' exponents,
// so 1.5 + 0.25 is 1.75 and 1.50 + 1 is 2.50.
func (x Decimal64) Add(y Decimal64) Decimal64 {
	var c Context
	return c.Add64(x, y)
}

// Sub returns the difference x - y, rounded to nearest even.
func (x Decimal64) Sub(y Decimal64) Decimal64 {
	var c Context
	return c.Sub64(x, y)
}

// Mul returns the product x × y, rounded to nearest even.
//
// The exponent of an exact product is the sum of the operands' exponents,
// so 1.5 × 2.00 is 3.000.
func (x Decimal64) Mul(y Decimal64) Decimal64 {
	var c Context
	return c.Mul64(x, y)
}

// Quo returns the quotient x / y, rounded to nearest even. Division of a
// finite non-zero number by zero returns an infinity.
//
// The exponent of an exact quotient is as close to the difference of the
// operands' exponents as the result allows, so 6.0 / 2 is 3.0 and 1 / 4 is
// 0.25.
func (x Decimal64) Quo(y Decimal64) Decimal64 {
	var c Context
	return c.Quo64(x, y)
}
