package decimal

import (
	"encoding/binary"
	"errors"
)

// Decimal128 is an IEEE 754 decimal128 floating-point number: 34 significant
// decimal digits with an exponent range of [-6143, 6144].
//
// The zero value is +0E+0. A Decimal128 is a plain 16-byte value; copying it
// is cheap and it never allocates. Like the other decimal types it
// deliberately does not support ==; see Decimal64.
type Decimal128 struct {
	_ [0]func() // not comparable

	// hi and lo are the IEEE 754 BID interchange encoding, with hi XORed
	// with zero128 so that the zero value of the struct is +0E+0 rather
	// than +0E-6176.
	hi, lo uint64
}

const (
	sign128    = 1 << 63
	zero128    = bias128 << 49      // high word of the BID encoding of +0E+0
	special128 = 0x6000000000000000 // set: large-coefficient form, Inf or NaN
	inf128     = 0x7800000000000000
	nan128     = 0x7C00000000000000
	snan128    = 0x7E00000000000000

	bias128 = 6176
)

// New128FromBits returns the Decimal128 whose IEEE 754 binary integer
// decimal (BID) interchange encoding has the high word hi and the low word
// lo. Every bit pattern is valid; non-canonical encodings are accepted and
// interpreted as the standard requires.
func New128FromBits(hi, lo uint64) Decimal128 { return Decimal128{hi: hi ^ zero128, lo: lo} }

// Bits returns the high and low words of the IEEE 754 binary integer decimal
// (BID) interchange encoding of x, exactly as stored: Bits does not
// canonicalize.
func (x Decimal128) Bits() (hi, lo uint64) { return x.hi ^ zero128, x.lo }

// unpack decodes x for the kernel.
func (x Decimal128) unpack() num128 {
	hi := x.hi ^ zero128
	n := num128{neg: hi>>63 != 0}
	switch {
	case hi&special128 != special128:
		n.coef = uint128{hi & (1<<49 - 1), x.lo}
		if maxCoef128.less(n.coef) {
			n.coef = uint128{} // non-canonical
		}
		n.exp = int32(hi>>49&0x3FFF) - bias128
	case hi&inf128 != inf128:
		// Large-coefficient form: with its implied 0b100 prefix the
		// coefficient always exceeds 10**34-1, a non-canonical zero.
		n.exp = int32(hi>>47&0x3FFF) - bias128
	case hi&nan128 == inf128:
		n.kind = infinite
	default:
		n.kind = quietNaN
		if hi&snan128 == snan128 {
			n.kind = signalingNaN
		}
		n.coef = uint128{hi & (1<<46 - 1), x.lo}
		if maxPayload128.less(n.coef) {
			n.coef = uint128{}
		}
	}
	return n
}

// pack128 encodes a kernel result, which is always in range and canonical.
func pack128(n num128) Decimal128 {
	hi := n.coef.hi
	switch n.kind {
	case finite:
		hi |= uint64(n.exp+bias128) << 49
	case infinite:
		hi = inf128
	case quietNaN:
		hi |= nan128
	default:
		hi |= snan128
	}
	if n.kind == infinite {
		n.coef.lo = 0
	}
	if n.neg {
		hi |= sign128
	}
	return Decimal128{hi: hi ^ zero128, lo: n.coef.lo}
}

// Inf128 returns positive infinity if sign >= 0, negative infinity if
// sign < 0.
func Inf128(sign int) Decimal128 {
	return pack128(num128{kind: infinite, neg: sign < 0})
}

// NaN128 returns a quiet NaN with a zero payload.
func NaN128() Decimal128 { return pack128(num128{kind: quietNaN}) }

// NewNaN128 returns a NaN carrying the given diagnostic payload.
func NewNaN128(payload uint64, signaling bool) Decimal128 {
	n := num128{kind: quietNaN, coef: uint128{0, payload}}
	if signaling {
		n.kind = signalingNaN
	}
	return pack128(n)
}

// IsNaN reports whether x is a NaN, quiet or signaling.
func (x Decimal128) IsNaN() bool { return (x.hi^zero128)&nan128 == nan128 }

// IsSignaling reports whether x is a signaling NaN.
func (x Decimal128) IsSignaling() bool { return (x.hi^zero128)&snan128 == snan128 }

// IsInf reports whether x is an infinity, according to sign. If sign > 0,
// IsInf reports whether x is positive infinity. If sign < 0, IsInf reports
// whether x is negative infinity. If sign == 0, IsInf reports whether x is
// either infinity.
func (x Decimal128) IsInf(sign int) bool {
	hi := x.hi ^ zero128
	return hi&nan128 == inf128 && (sign == 0 || (sign < 0) == (hi>>63 != 0))
}

// IsFinite reports whether x is neither infinite nor NaN.
func (x Decimal128) IsFinite() bool { return (x.hi^zero128)&inf128 != inf128 }

// IsZero reports whether x is +0 or -0, with any exponent.
func (x Decimal128) IsZero() bool { return x.unpack().isZero() }

// IsNormal reports whether x is finite, non-zero and not subnormal.
func (x Decimal128) IsNormal() bool { return x.unpack().isNormal() }

// IsSubnormal reports whether x is non-zero with magnitude less than the
// smallest normal number, 1E-6143.
func (x Decimal128) IsSubnormal() bool { return x.unpack().isSubnormal() }

// IsInteger reports whether x is finite and has no fractional part.
func (x Decimal128) IsInteger() bool { return x.unpack().isInteger() }

// Signbit reports whether the sign bit of x is set. It is set for negative
// numbers and negative zero, and may be set for NaNs.
func (x Decimal128) Signbit() bool { return x.hi>>63 != 0 }

// Sign returns -1 if x < 0, 0 if x is ±0 or NaN, and +1 if x > 0.
func (x Decimal128) Sign() int { return x.unpack().sign() }

// IsCanonical reports whether the encoding of x is canonical. The results of
// all arithmetic operations are canonical; non-canonical values can only
// enter through New128FromBits.
func (x Decimal128) IsCanonical() bool {
	c := pack128(x.unpack())
	return c.hi == x.hi && c.lo == x.lo
}

// Canonical returns x with a canonical encoding. The value, sign, exponent
// and any NaN payload that fits are unchanged.
func (x Decimal128) Canonical() Decimal128 { return pack128(x.unpack()) }

// Class returns the IEEE 754 class of x.
func (x Decimal128) Class() Class { return x.unpack().class() }

// Neg returns x with its sign inverted. Like Abs and CopySign it is a quiet
// operation: it affects only the sign bit, even of a NaN, and raises no
// exception.
func (x Decimal128) Neg() Decimal128 { return Decimal128{hi: x.hi ^ sign128, lo: x.lo} }

// Abs returns x with its sign bit cleared.
func (x Decimal128) Abs() Decimal128 { return Decimal128{hi: x.hi &^ sign128, lo: x.lo} }

// CopySign returns x with the sign of y.
func (x Decimal128) CopySign(y Decimal128) Decimal128 {
	return Decimal128{hi: x.hi&^sign128 | y.hi&sign128, lo: x.lo}
}

// Parts returns the sign, coefficient and exponent of a finite x, such that
// x = (-1)**neg × coef × 10**exp, where the coefficient coef is
// hi × 2**64 + lo. For infinities and NaNs, the coefficient is zero (or the
// NaN payload) and exp is zero; use IsFinite to tell them apart.
func (x Decimal128) Parts() (neg bool, hi, lo uint64, exp int) {
	n := x.unpack()
	return n.neg, n.coef.hi, n.coef.lo, int(n.exp)
}

// Exponent returns the exponent q of x in the representation
// coefficient × 10**q, or 0 if x is not finite.
func (x Decimal128) Exponent() int { return int(x.unpack().exp) }

// AppendBinary implements encoding.BinaryAppender. The encoding is the 16
// bytes of the BID interchange encoding, most significant byte first.
func (x Decimal128) AppendBinary(b []byte) ([]byte, error) {
	hi, lo := x.Bits()
	return binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(b, hi), lo), nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (x Decimal128) MarshalBinary() ([]byte, error) { return x.AppendBinary(nil) }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (x *Decimal128) UnmarshalBinary(b []byte) error {
	if len(b) != 16 {
		return errors.New("decimal: invalid Decimal128 binary encoding")
	}
	*x = New128FromBits(binary.BigEndian.Uint64(b), binary.BigEndian.Uint64(b[8:]))
	return nil
}

// New128FromDPD returns the Decimal128 whose IEEE 754 densely packed decimal
// interchange encoding has the high word hi and the low word lo. Every bit
// pattern is valid.
func New128FromDPD(hi, lo uint64) Decimal128 {
	n := num128{neg: hi>>63 != 0}
	// The 110-bit coefficient continuation holds 11 declets.
	cont := uint128{hi & (1<<46 - 1), lo}
	var digits uint128
	for i := 10; i >= 0; i-- {
		digits = digits.mul64(1000).add64(uint64(dpd2bin[cont.rsh(uint(10*i)).lo&0x3FF]))
	}
	comb := hi >> 58 & 0x1F
	switch {
	case comb == 0b11110:
		n.kind = infinite
	case comb == 0b11111:
		n.kind = quietNaN
		if hi>>57&1 != 0 {
			n.kind = signalingNaN
		}
		n.coef = digits
	default:
		lead, expMSB := dpdSplit(comb)
		n.coef = pow10tab128[33].mul64(lead).add(digits)
		n.exp = int32(expMSB<<12|hi>>46&0xFFF) - bias128
	}
	return pack128(n)
}

// DPD returns the high and low words of the IEEE 754 densely packed decimal
// interchange encoding of x, which is always canonical.
func (x Decimal128) DPD() (hi, lo uint64) {
	n := x.unpack()
	lead, rest := n.coef.quoRem(pow10tab128[33])
	var cont uint128
	for i := range 11 {
		var r uint64
		rest, r = rest.quoRemPow10(3)
		d := uint128{0, uint64(bin2dpd[r])}.lsh(uint(10 * i))
		cont.hi |= d.hi
		cont.lo |= d.lo
	}
	hi, lo = cont.hi, cont.lo
	switch n.kind {
	case finite:
		e := uint64(n.exp + bias128)
		hi |= dpdCombination(lead.lo, e>>12)<<58 | e&0xFFF<<46
	case infinite:
		hi, lo = 0b11110<<58, 0
	case quietNaN:
		hi |= 0b11111 << 58
	default:
		hi |= 0b11111<<58 | 1<<57
	}
	if n.neg {
		hi |= sign128
	}
	return hi, lo
}
