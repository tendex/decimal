// Package decimal implements the decimal floating-point arithmetic of
// IEEE 754: the fixed-size formats decimal32, decimal64 and decimal128, as
// the value types [Decimal32], [Decimal64] and [Decimal128].
//
// Decimal floating point represents numbers such as 0.1 and 19.99 exactly,
// rounds the way people do, and remembers how many decimal places a number
// was written with. It is the arithmetic of money, measurement and anything
// else whose inputs and outputs are decimal; binary floating point (float64)
// can only approximate it.
//
//	price := decimal.MustParse64("19.99")
//	total := price.Mul(decimal.New64(3, 0)) // 59.97, exactly
//
// # Values
//
// The three types are plain values of 4, 8 and 16 bytes holding 7, 16 and 34
// significant digits. Arithmetic never allocates, the zero value is the number
// zero, and every method is safe for concurrent use. All three have the
// same methods; Decimal64 is the natural choice for most purposes, and the
// one whose documentation is the most complete.
//
// A finite number is a sign, an integer coefficient and an exponent,
// coefficient × 10**exponent. Unlike binary floating point the
// representation is not normalized: 1, 1.0 and 1.00 are three different
// members of one cohort, equal in value but with different exponents.
// Arithmetic preserves exponents the way pencil-and-paper arithmetic does
// (1.20 + 1.1 is 2.30, and 1.5 × 2.00 is 3.000), which is how results keep
// the precision their inputs were given with.
//
// It is also why Decimal64 and Decimal128 do not support the == operator: it
// would compare representations rather than values. Compare with
// [Decimal64.Equal], [Decimal64.Less] or [Decimal64.Cmp], all of which treat
// cohort members as equal. [Decimal64.CmpTotal] orders representations, and
// [Decimal64.Reduce] picks one canonical member of each cohort.
//
// # Rounding and exceptions
//
// Every operation is correctly rounded: the result is the exact result
// rounded once to the precision of the format. The methods on the decimal
// types round to nearest, ties to even, and deliver the default results
// IEEE 754 specifies for exceptional cases, such as an infinity for 1/0 and
// a NaN for 0/0.
//
// A [Context] provides the other rounding directions and reports the five
// IEEE 754 exceptions as sticky flags:
//
//	c := decimal.Context{Rounding: decimal.ToZero}
//	q := c.Quo64(x, y)
//	if c.Flags&decimal.Inexact != 0 {
//		// q was truncated
//	}
//
// Context methods are named for the operation and the width of the format,
// as in Add64 and Sqrt128. Every method on a decimal type that can round or
// raise an exception has a Context counterpart.
//
// # Conformance
//
// The package implements all the operations IEEE 754-2019 requires of a
// decimal format: the arithmetic operations including fused multiply-add,
// square root and remainder; quantize and quantum; conversions to and from
// integers, binary floating point, the other decimal formats and character
// sequences, correctly rounded for inputs of any length; comparisons, total
// order and classification; and both interchange encodings. Subnormal
// numbers, signed zeros, infinities, quiet and signaling NaNs with payloads,
// and non-canonical encodings are all handled as specified.
//
// It is tested against the General Decimal Arithmetic test cases of Mike
// Cowlishaw, the de facto conformance suite for these formats, and against
// an arbitrary-precision reference implementation on random operands.
//
// Alternate exception handling (traps) and the transcendental functions
// that IEEE 754 merely recommends are not provided.
//
// # Encodings
//
// Values are held in the binary integer decimal (BID) interchange encoding,
// available through Bits and the FromBits constructors. The densely packed
// decimal (DPD) encoding used by IBM systems is available through DPD and
// the FromDPD constructors. The text form produced by String and accepted
// by the Parse functions round-trips every value exactly, and is also the
// form used by the encoding.TextMarshaler implementations, and therefore by
// encoding/json. Scan and Value implement sql.Scanner and driver.Valuer,
// so that the types can be read from and written to database/sql columns;
// sql.Null[Decimal64] holds one that may be NULL.
package decimal

// The Decimal32 and Decimal128 method sets are derived from Decimal64's.
//go:generate go run gen_formats.go
