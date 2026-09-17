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
