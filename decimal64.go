package decimal

import (
	"encoding/binary"
	"errors"
)

// Decimal64 is an IEEE 754 decimal64 floating-point number: 16 significant
// decimal digits with an exponent range of [-383, 384].
//
// The zero value is +0E+0. A Decimal64 is a plain 8-byte value; copying it is
// cheap and it never allocates.
//
// Decimal64 deliberately does not support ==. Decimal formats are redundant:
// 1, 1.0 and 1.00 are distinct members of the same cohort, equal in value but
// not in representation, so a bitwise comparison would be wrong. Use Equal,
// Cmp or the Compare methods instead; use CmpTotal, or Canonical and Bits,
// when representations themselves must be distinguished or used as map keys.
type Decimal64 struct {
	_ [0]func() // not comparable

	// bits is the IEEE 754 BID interchange encoding XORed with zero64, so
	// that the zero value of the struct is +0E+0 rather than the much less
	// convenient +0E-398.
	bits uint64
}

const (
	sign64    = 1 << 63
	zero64    = bias64 << 53       // BID encoding of +0E+0
	special64 = 0x6000000000000000 // set: large-coefficient form, Inf or NaN
	inf64     = 0x7800000000000000
	nan64     = 0x7C00000000000000
	snan64    = 0x7E00000000000000

	bias64       = 398
	maxCoef64    = 9999999999999999 // 10**16 - 1
	maxPayload64 = 999999999999999  // 10**15 - 1
)

// format64 describes decimal64 to the 64-bit kernel.
var format64 = format{prec: 16, emax: 369, emin: -398, maxCoef: maxCoef64, maxPayload: maxPayload64}

// New64FromBits returns the Decimal64 with the IEEE 754 binary integer
// decimal (BID) interchange encoding b. Every bit pattern is valid;
// non-canonical encodings are accepted and interpreted as the standard
// requires.
func New64FromBits(b uint64) Decimal64 { return Decimal64{bits: b ^ zero64} }

// Bits returns the IEEE 754 binary integer decimal (BID) interchange encoding
// of x, exactly as stored: Bits does not canonicalize.
func (x Decimal64) Bits() uint64 { return x.bits ^ zero64 }

// unpack decodes x for the kernel.
func (x Decimal64) unpack() num {
	b := x.bits ^ zero64
	if b&special64 != special64 {
		return num{coef: b & (1<<53 - 1), exp: int32(b>>53&0x3FF) - bias64, neg: b>>63 != 0}
	}
	return unpackSpecial64(b)
}

func unpackSpecial64(b uint64) num {
	n := num{neg: b>>63 != 0}
	switch {
	case b&inf64 != inf64:
		// Large-coefficient form: the coefficient has an implied 0b100
		// prefix. Values above 10**16-1 are non-canonical zeros.
		n.coef = 1<<53 | b&(1<<51-1)
		if n.coef > maxCoef64 {
			n.coef = 0
		}
		n.exp = int32(b>>51&0x3FF) - bias64
	case b&nan64 == inf64:
		n.kind = infinite
	default:
		n.kind = quietNaN
		if b&snan64 == snan64 {
			n.kind = signalingNaN
		}
		n.coef = b & (1<<50 - 1)
		if n.coef > maxPayload64 {
			n.coef = 0
		}
	}
	return n
}

// pack64 encodes a kernel result, which is always in range and canonical.
func pack64(n num) Decimal64 {
	var b uint64
	switch n.kind {
	case finite:
		if n.coef < 1<<53 {
			b = uint64(n.exp+bias64)<<53 | n.coef
		} else {
			b = special64 | uint64(n.exp+bias64)<<51 | n.coef&(1<<51-1)
		}
	case infinite:
		b = inf64
	case quietNaN:
		b = nan64 | n.coef
	default:
		b = snan64 | n.coef
	}
	if n.neg {
		b |= sign64
	}
	return Decimal64{bits: b ^ zero64}
}

// Inf64 returns positive infinity if sign >= 0, negative infinity if
// sign < 0.
func Inf64(sign int) Decimal64 {
	return pack64(num{kind: infinite, neg: sign < 0})
}

// NaN64 returns a quiet NaN with a zero payload.
func NaN64() Decimal64 { return pack64(num{kind: quietNaN}) }

// NewNaN64 returns a NaN carrying the given diagnostic payload. Payloads
// larger than 10**15-1 do not fit and are replaced by zero.
func NewNaN64(payload uint64, signaling bool) Decimal64 {
	n := num{kind: quietNaN, coef: payload}
	if signaling {
		n.kind = signalingNaN
	}
	if n.coef > maxPayload64 {
		n.coef = 0
	}
	return pack64(n)
}

// IsNaN reports whether x is a NaN, quiet or signaling.
func (x Decimal64) IsNaN() bool { return (x.bits^zero64)&nan64 == nan64 }

// IsSignaling reports whether x is a signaling NaN.
func (x Decimal64) IsSignaling() bool { return (x.bits^zero64)&snan64 == snan64 }

// IsInf reports whether x is an infinity, according to sign. If sign > 0,
// IsInf reports whether x is positive infinity. If sign < 0, IsInf reports
// whether x is negative infinity. If sign == 0, IsInf reports whether x is
// either infinity.
func (x Decimal64) IsInf(sign int) bool {
	b := x.bits ^ zero64
	return b&nan64 == inf64 && (sign == 0 || (sign < 0) == (b>>63 != 0))
}

// IsFinite reports whether x is neither infinite nor NaN.
func (x Decimal64) IsFinite() bool { return (x.bits^zero64)&inf64 != inf64 }

// IsZero reports whether x is +0 or -0, with any exponent.
func (x Decimal64) IsZero() bool {
	n := x.unpack()
	return n.kind == finite && n.coef == 0
}

// IsNormal reports whether x is finite, non-zero and not subnormal.
func (x Decimal64) IsNormal() bool { return x.unpack().isNormal(&format64) }

// IsSubnormal reports whether x is non-zero with magnitude less than the
// smallest normal number, 1E-383.
func (x Decimal64) IsSubnormal() bool { return x.unpack().isSubnormal(&format64) }

// IsInteger reports whether x is finite and has no fractional part.
func (x Decimal64) IsInteger() bool { return x.unpack().isInteger() }

// Signbit reports whether the sign bit of x is set. It is set for negative
// numbers and negative zero, and may be set for NaNs.
func (x Decimal64) Signbit() bool { return (x.bits^zero64)>>63 != 0 }

// Sign returns -1 if x < 0, 0 if x is ±0 or NaN, and +1 if x > 0.
func (x Decimal64) Sign() int { return x.unpack().sign() }

// IsCanonical reports whether the encoding of x is canonical. The results of
// all arithmetic operations are canonical; non-canonical values can only
// enter through New64FromBits.
func (x Decimal64) IsCanonical() bool { return pack64(x.unpack()).bits == x.bits }

// Canonical returns x with a canonical encoding. The value, sign, exponent
// and any NaN payload that fits are unchanged.
func (x Decimal64) Canonical() Decimal64 { return pack64(x.unpack()) }

// Class returns the IEEE 754 class of x.
func (x Decimal64) Class() Class { return x.unpack().class(&format64) }

// Neg returns x with its sign inverted. Like Abs and CopySign it is a quiet
// operation: it affects only the sign bit, even of a NaN, and raises no
// exception.
func (x Decimal64) Neg() Decimal64 { return Decimal64{bits: x.bits ^ sign64} }

// Abs returns x with its sign bit cleared.
func (x Decimal64) Abs() Decimal64 { return Decimal64{bits: x.bits &^ sign64} }

// CopySign returns x with the sign of y.
func (x Decimal64) CopySign(y Decimal64) Decimal64 {
	return Decimal64{bits: x.bits&^sign64 | y.bits&sign64}
}

// Parts returns the sign, coefficient and exponent of a finite x, such that
// x = (-1)**neg × coef × 10**exp. For infinities and NaNs, coef is zero (or
// the NaN payload) and exp is zero; use IsFinite to tell them apart.
func (x Decimal64) Parts() (neg bool, coef uint64, exp int) {
	n := x.unpack()
	return n.neg, n.coef, int(n.exp)
}

// Exponent returns the exponent q of x in the representation
// coefficient × 10**q, or 0 if x is not finite.
func (x Decimal64) Exponent() int { return int(x.unpack().exp) }

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
