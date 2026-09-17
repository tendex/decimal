package decimal

import (
	"encoding/binary"
	"errors"
)

// Decimal32 is an IEEE 754 decimal32 floating-point number: 7 significant
// decimal digits with an exponent range of [-95, 96].
//
// IEEE 754 defines decimal32 as an interchange format, meant for compact
// storage rather than computation. This package nevertheless gives it the
// same complete, correctly rounded arithmetic as the larger formats.
//
// The zero value is +0E+0. A Decimal32 is a plain 4-byte value.
//
// Unlike Decimal64 and Decimal128, Decimal32 cannot be made to reject ==
// without doubling its size, which would defeat its purpose. Do not use ==
// on Decimal32 values: it compares representations, so 1.0 == 1.00 is false
// and NaN == NaN may be true. Use Equal, Cmp or CmpTotal; see Decimal64.
type Decimal32 struct {
	// bits is the IEEE 754 BID interchange encoding XORed with zero32, so
	// that the zero value of the struct is +0E+0 rather than +0E-101.
	bits uint32
}

const (
	sign32    = 1 << 31
	zero32    = bias32 << 23 // BID encoding of +0E+0
	special32 = 0x60000000   // set: large-coefficient form, Inf or NaN
	inf32     = 0x78000000
	nan32     = 0x7C000000
	snan32    = 0x7E000000

	bias32       = 101
	maxCoef32    = 9999999 // 10**7 - 1
	maxPayload32 = 999999  // 10**6 - 1
)

// format32 describes decimal32 to the 64-bit kernel.
var format32 = format{prec: 7, emax: 90, emin: -101, maxCoef: maxCoef32, maxPayload: maxPayload32}

// New32FromBits returns the Decimal32 with the IEEE 754 binary integer
// decimal (BID) interchange encoding b. Every bit pattern is valid;
// non-canonical encodings are accepted and interpreted as the standard
// requires.
func New32FromBits(b uint32) Decimal32 { return Decimal32{bits: b ^ zero32} }

// Bits returns the IEEE 754 binary integer decimal (BID) interchange encoding
// of x, exactly as stored: Bits does not canonicalize.
func (x Decimal32) Bits() uint32 { return x.bits ^ zero32 }

// unpack decodes x for the kernel.
func (x Decimal32) unpack() num {
	b := x.bits ^ zero32
	if b&special32 != special32 {
		return num{coef: uint64(b & (1<<23 - 1)), exp: int32(b>>23&0xFF) - bias32, neg: b>>31 != 0}
	}
	n := num{neg: b>>31 != 0}
	switch {
	case b&inf32 != inf32:
		// Large-coefficient form: the coefficient has an implied 0b100
		// prefix. Values above 10**7-1 are non-canonical zeros.
		n.coef = uint64(1<<23 | b&(1<<21-1))
		if n.coef > maxCoef32 {
			n.coef = 0
		}
		n.exp = int32(b>>21&0xFF) - bias32
	case b&nan32 == inf32:
		n.kind = infinite
	default:
		n.kind = quietNaN
		if b&snan32 == snan32 {
			n.kind = signalingNaN
		}
		n.coef = uint64(b & (1<<20 - 1))
		if n.coef > maxPayload32 {
			n.coef = 0
		}
	}
	return n
}

// pack32 encodes a kernel result, which is always in range and canonical.
func pack32(n num) Decimal32 {
	var b uint32
	switch n.kind {
	case finite:
		if n.coef < 1<<23 {
			b = uint32(n.exp+bias32)<<23 | uint32(n.coef)
		} else {
			b = special32 | uint32(n.exp+bias32)<<21 | uint32(n.coef)&(1<<21-1)
		}
	case infinite:
		b = inf32
	case quietNaN:
		b = nan32 | uint32(n.coef)
	default:
		b = snan32 | uint32(n.coef)
	}
	if n.neg {
		b |= sign32
	}
	return Decimal32{bits: b ^ zero32}
}

// Inf32 returns positive infinity if sign >= 0, negative infinity if
// sign < 0.
func Inf32(sign int) Decimal32 {
	return pack32(num{kind: infinite, neg: sign < 0})
}

// NaN32 returns a quiet NaN with a zero payload.
func NaN32() Decimal32 { return pack32(num{kind: quietNaN}) }

// NewNaN32 returns a NaN carrying the given diagnostic payload. Payloads
// larger than 10**6-1 do not fit and are replaced by zero.
func NewNaN32(payload uint64, signaling bool) Decimal32 {
	n := num{kind: quietNaN, coef: payload}
	if signaling {
		n.kind = signalingNaN
	}
	if n.coef > maxPayload32 {
		n.coef = 0
	}
	return pack32(n)
}

// IsNaN reports whether x is a NaN, quiet or signaling.
func (x Decimal32) IsNaN() bool { return (x.bits^zero32)&nan32 == nan32 }

// IsSignaling reports whether x is a signaling NaN.
func (x Decimal32) IsSignaling() bool { return (x.bits^zero32)&snan32 == snan32 }

// IsInf reports whether x is an infinity, according to sign. If sign > 0,
// IsInf reports whether x is positive infinity. If sign < 0, IsInf reports
// whether x is negative infinity. If sign == 0, IsInf reports whether x is
// either infinity.
func (x Decimal32) IsInf(sign int) bool {
	b := x.bits ^ zero32
	return b&nan32 == inf32 && (sign == 0 || (sign < 0) == (b>>31 != 0))
}

// IsFinite reports whether x is neither infinite nor NaN.
func (x Decimal32) IsFinite() bool { return (x.bits^zero32)&inf32 != inf32 }

// IsZero reports whether x is +0 or -0, with any exponent.
func (x Decimal32) IsZero() bool {
	n := x.unpack()
	return n.kind == finite && n.coef == 0
}

// IsNormal reports whether x is finite, non-zero and not subnormal.
func (x Decimal32) IsNormal() bool { return x.unpack().isNormal(&format32) }

// IsSubnormal reports whether x is non-zero with magnitude less than the
// smallest normal number, 1E-95.
func (x Decimal32) IsSubnormal() bool { return x.unpack().isSubnormal(&format32) }

// IsInteger reports whether x is finite and has no fractional part.
func (x Decimal32) IsInteger() bool { return x.unpack().isInteger() }

// Signbit reports whether the sign bit of x is set. It is set for negative
// numbers and negative zero, and may be set for NaNs.
func (x Decimal32) Signbit() bool { return (x.bits^zero32)>>31 != 0 }

// Sign returns -1 if x < 0, 0 if x is ±0 or NaN, and +1 if x > 0.
func (x Decimal32) Sign() int { return x.unpack().sign() }

// IsCanonical reports whether the encoding of x is canonical. The results of
// all arithmetic operations are canonical; non-canonical values can only
// enter through New32FromBits.
func (x Decimal32) IsCanonical() bool { return pack32(x.unpack()).bits == x.bits }

// Canonical returns x with a canonical encoding. The value, sign, exponent
// and any NaN payload that fits are unchanged.
func (x Decimal32) Canonical() Decimal32 { return pack32(x.unpack()) }

// Class returns the IEEE 754 class of x.
func (x Decimal32) Class() Class { return x.unpack().class(&format32) }

// Neg returns x with its sign inverted. Like Abs and CopySign it is a quiet
// operation: it affects only the sign bit, even of a NaN, and raises no
// exception.
func (x Decimal32) Neg() Decimal32 { return Decimal32{bits: x.bits ^ sign32} }

// Abs returns x with its sign bit cleared.
func (x Decimal32) Abs() Decimal32 { return Decimal32{bits: x.bits &^ sign32} }

// CopySign returns x with the sign of y.
func (x Decimal32) CopySign(y Decimal32) Decimal32 {
	return Decimal32{bits: x.bits&^sign32 | y.bits&sign32}
}

// Parts returns the sign, coefficient and exponent of a finite x, such that
// x = (-1)**neg × coef × 10**exp. For infinities and NaNs, coef is zero (or
// the NaN payload) and exp is zero; use IsFinite to tell them apart.
func (x Decimal32) Parts() (neg bool, coef uint64, exp int) {
	n := x.unpack()
	return n.neg, n.coef, int(n.exp)
}

// Exponent returns the exponent q of x in the representation
// coefficient × 10**q, or 0 if x is not finite.
func (x Decimal32) Exponent() int { return int(x.unpack().exp) }

// AppendBinary implements encoding.BinaryAppender. The encoding is the 4
// bytes of the BID interchange encoding, most significant byte first.
func (x Decimal32) AppendBinary(b []byte) ([]byte, error) {
	return binary.BigEndian.AppendUint32(b, x.Bits()), nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (x Decimal32) MarshalBinary() ([]byte, error) { return x.AppendBinary(nil) }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (x *Decimal32) UnmarshalBinary(b []byte) error {
	if len(b) != 4 {
		return errors.New("decimal: invalid Decimal32 binary encoding")
	}
	*x = New32FromBits(binary.BigEndian.Uint32(b))
	return nil
}

// New32FromDPD returns the Decimal32 with the IEEE 754 densely packed
// decimal interchange encoding b. Every bit pattern is valid.
func New32FromDPD(b uint32) Decimal32 {
	n := num{neg: b>>31 != 0}
	comb := uint64(b >> 26 & 0x1F)
	switch {
	case comb == 0b11110:
		n.kind = infinite
	case comb == 0b11111:
		n.kind = quietNaN
		if b>>25&1 != 0 {
			n.kind = signalingNaN
		}
		n.coef = undeclets(uint64(b), 2)
	default:
		lead, expMSB := dpdSplit(comb)
		n.coef = lead*1e6 + undeclets(uint64(b), 2)
		n.exp = int32(expMSB<<6|uint64(b>>20&0x3F)) - bias32
	}
	return pack32(n)
}

// DPD returns the IEEE 754 densely packed decimal interchange encoding of x,
// which is always canonical.
func (x Decimal32) DPD() uint32 {
	n := x.unpack()
	var b uint64
	switch n.kind {
	case finite:
		e := uint64(n.exp + bias32)
		b = dpdCombination(n.coef/1e6, e>>6)<<26 | e&0x3F<<20 | declets(n.coef%1e6, 2)
	case infinite:
		b = 0b11110 << 26
	case quietNaN:
		b = 0b11111<<26 | declets(n.coef, 2)
	default:
		b = 0b11111<<26 | 1<<25 | declets(n.coef, 2)
	}
	if n.neg {
		b |= sign32
	}
	return uint32(b)
}
