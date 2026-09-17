package decimal

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Parse64 converts the string s to a Decimal64, rounding to nearest even if
// s has more than 16 significant digits.
//
// Parse64 accepts decimal numbers in plain or scientific notation
// ("-12.50", ".5", "1.25E+3"), infinities ("Inf", "-Infinity") and NaNs
// ("NaN", "sNaN", either optionally followed by a payload as in "NaN123"),
// all without regard to case. The exponent of the result follows the text,
// so "1.50" and "1.5" parse to different members of the same cohort.
//
// If s is not syntactically well-formed, Parse64 returns a quiet NaN and an
// error of type *strconv.NumError wrapping strconv.ErrSyntax. Overflow and
// underflow are not errors; use Context.Parse64 to observe them.
func Parse64(s string) (Decimal64, error) {
	var c Context
	return c.Parse64(s)
}

// MustParse64 is like Parse64 but panics if s cannot be parsed. It
// simplifies the initialization of package-level variables.
func MustParse64(s string) Decimal64 {
	x, err := Parse64(s)
	if err != nil {
		panic(err)
	}
	return x
}

// Parse64 is like the package-level Parse64 but rounds according to the
// context and raises Inexact, Overflow and Underflow as appropriate, and
// Invalid for a syntax error.
func (c *Context) Parse64(s string) (Decimal64, error) {
	n, err := c.parse(&format64, "Parse64", s)
	if err != nil {
		c.Flags |= Invalid
	}
	return pack64(n), err
}

func (x Decimal64) text() *text {
	n := x.unpack()
	t := &text{neg: n.neg, kind: n.kind}
	t.setCoef(uint128{0, n.coef}, int(n.exp))
	return t
}

// String returns x in the scientific notation defined by IEEE 754 and the
// General Decimal Arithmetic specification: plain digits when the exponent
// is small ("-12.50", "0.000001"), otherwise one digit before the point and
// an exponent ("1.25E+30"). Infinities are "Infinity" and "-Infinity"; NaNs
// are "NaN" or "sNaN", followed by the payload if it is non-zero.
//
// The string preserves the exponent, so Parse64(x.String()) recovers x
// exactly, including its position in its cohort.
func (x Decimal64) String() string { return string(x.text().appendSci(nil)) }

// Text converts x to a string in the given strconv-style format: 'e' or 'E'
// (-d.dddde±dd), 'f' or 'F' (-ddd.ddd), or 'g' or 'G' ('e' for large
// exponents, 'f' otherwise). The precision prec is the number of digits
// after the decimal point for 'e' and 'f' and the number of significant
// digits for 'g'; rounding to it is to nearest even. A negative precision
// formats x exactly, trailing zeros included.
func (x Decimal64) Text(format byte, prec int) string {
	return string(x.Append(nil, format, prec))
}

// Append appends the Text form of x to b and returns the extended buffer.
func (x Decimal64) Append(b []byte, format byte, prec int) []byte {
	return x.text().append(b, format, prec)
}

// Format implements fmt.Formatter. It accepts %v and %s (as String), and
// %e, %E, %f, %F, %g and %G (as Text, with fmt's default precision of 6 for
// %e and %f), along with the usual width, precision and '+', '-', ' ' and
// '0' flags.
func (x Decimal64) Format(s fmt.State, verb rune) { x.text().format(s, verb, "Decimal64") }

// AppendText implements encoding.TextAppender using the String form.
func (x Decimal64) AppendText(b []byte) ([]byte, error) { return x.text().appendSci(b), nil }

// MarshalText implements encoding.TextMarshaler using the String form.
func (x Decimal64) MarshalText() ([]byte, error) { return x.AppendText(nil) }

// UnmarshalText implements encoding.TextUnmarshaler. It accepts the same
// syntax as Parse64.
func (x *Decimal64) UnmarshalText(b []byte) error {
	v, err := Parse64(string(b))
	if err != nil {
		return err
	}
	*x = v
	return nil
}

// AppendBinary implements encoding.BinaryAppender. The encoding is the 8
// bytes of the BID interchange encoding, most significant byte first.
func (x Decimal64) AppendBinary(b []byte) ([]byte, error) {
	return binary.BigEndian.AppendUint64(b, x.Bits()), nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (x Decimal64) MarshalBinary() ([]byte, error) { return x.AppendBinary(nil) }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (x *Decimal64) UnmarshalBinary(b []byte) error {
	if len(b) != 8 {
		return errors.New("decimal: invalid Decimal64 binary encoding")
	}
	*x = New64FromBits(binary.BigEndian.Uint64(b))
	return nil
}
