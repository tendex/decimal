package decimal

import (
	"fmt"
	"math/big"
	"math/rand/v2"
)

// This file is a reference implementation of correctly rounded decimal
// arithmetic on math/big integers. It is slow and obvious where the package
// proper is fast and careful, and the two are compared on random operands.

type refFormat struct {
	name             string
	prec, emax, emin int
}

var (
	ref32  = refFormat{"decimal32", 7, 90, -101}
	ref64  = refFormat{"decimal64", 16, 369, -398}
	ref128 = refFormat{"decimal128", 34, 6111, -6176}
)

// refVal is a finite number, infinity or NaN; coef is never nil.
type refVal struct {
	kind kind
	neg  bool
	coef *big.Int
	exp  int
}

func (v refVal) String() string {
	sign := ""
	if v.neg {
		sign = "-"
	}
	switch v.kind {
	case infinite:
		return sign + "Inf"
	case quietNaN:
		return sign + "NaN" + v.coef.String()
	case signalingNaN:
		return sign + "sNaN" + v.coef.String()
	}
	return fmt.Sprintf("%s%sE%+d", sign, v.coef, v.exp)
}

func (v refVal) same(w refVal) bool {
	return v.kind == w.kind && v.neg == w.neg && v.coef.Cmp(w.coef) == 0 && (v.kind != finite || v.exp == w.exp)
}

func refPow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

func refDigits(x *big.Int) int {
	if x.Sign() == 0 {
		return 0
	}
	return len(x.String())
}

// round rounds (coef + ε) × 10**exp, where ε is a positive fraction if
// sticky is set, to the format.
func (f refFormat) round(mode RoundingMode, neg bool, coef *big.Int, exp int, sticky bool) (refVal, Flags) {
	var flags Flags
	coef = new(big.Int).Set(coef)
	if coef.Sign() == 0 && !sticky {
		return refVal{neg: neg, coef: coef, exp: min(max(exp, f.emin), f.emax)}, 0
	}
	nd := refDigits(coef)
	tiny := exp+nd-1 < f.emin+f.prec-1
	twice := 0 // discarded part compared with half a unit: -1, 0, +1
	inexact := sticky
	if drop := max(nd-f.prec, f.emin-exp); drop > 0 {
		pow := refPow10(drop)
		rem := new(big.Int)
		coef.QuoRem(coef, pow, rem)
		exp += drop
		inexact = inexact || rem.Sign() != 0
		twice = rem.Lsh(rem, 1).Cmp(pow)
		if twice == 0 && sticky {
			twice = 1
		}
	} else {
		twice = -1
	}
	if inexact {
		flags |= Inexact
		if tiny {
			flags |= Underflow
		}
		var up bool
		switch mode {
		case ToNearestEven:
			up = twice > 0 || twice == 0 && coef.Bit(0) == 1
		case ToNearestAway:
			up = twice >= 0
		case ToPositiveInf:
			up = !neg
		case ToNegativeInf:
			up = neg
		}
		if up {
			coef.Add(coef, big.NewInt(1))
			if refDigits(coef) > f.prec {
				coef.Quo(coef, big.NewInt(10))
				exp++
			}
		}
	}
	if exp > f.emax {
		if k := exp - f.emax; coef.Sign() != 0 && refDigits(coef)+k > f.prec {
			flags |= Overflow | Inexact
			if mode == ToZero || mode == ToPositiveInf && neg || mode == ToNegativeInf && !neg {
				max := refPow10(f.prec)
				return refVal{neg: neg, coef: max.Sub(max, big.NewInt(1)), exp: f.emax}, flags
			}
			return refVal{kind: infinite, neg: neg, coef: new(big.Int)}, flags
		} else {
			coef.Mul(coef, refPow10(k))
			exp = f.emax
		}
	}
	return refVal{neg: neg, coef: coef, exp: exp}, flags
}

var refNaN = refVal{kind: quietNaN, coef: new(big.Int)}

// nan returns the propagated NaN among the operands, if any.
func refNaNOf(ops ...refVal) (refVal, Flags, bool) {
	for _, want := range []kind{signalingNaN, quietNaN} {
		for _, v := range ops {
			if v.kind == want {
				var fl Flags
				if want == signalingNaN {
					fl = Invalid
				}
				return refVal{kind: quietNaN, neg: v.neg, coef: v.coef}, fl, true
			}
		}
	}
	return refVal{}, 0, false
}

// signed returns the signed coefficient of v scaled to exponent exp, which
// must not exceed v.exp.
func (v refVal) signed(exp int) *big.Int {
	z := new(big.Int).Mul(v.coef, refPow10(v.exp-exp))
	if v.neg {
		z.Neg(z)
	}
	return z
}

// sum rounds the exact sum of finite terms, given the sign an exact zero
// result takes when the terms' signs would not settle it.
func (f refFormat) sum(mode RoundingMode, zeroNeg bool, terms ...refVal) (refVal, Flags) {
	exp := terms[0].exp
	for _, t := range terms {
		exp = min(exp, t.exp)
	}
	total := new(big.Int)
	for _, t := range terms {
		total.Add(total, t.signed(exp))
	}
	neg := total.Sign() < 0
	if total.Sign() == 0 {
		neg = zeroNeg
	}
	return f.round(mode, neg, total.Abs(total), exp, false)
}

func refInf(neg bool) refVal { return refVal{kind: infinite, neg: neg, coef: new(big.Int)} }

func (f refFormat) add(mode RoundingMode, x, y refVal) (refVal, Flags) {
	if n, fl, ok := refNaNOf(x, y); ok {
		return n, fl
	}
	switch {
	case x.kind == infinite && y.kind == infinite && x.neg != y.neg:
		return refNaN, Invalid
	case x.kind == infinite:
		return refInf(x.neg), 0
	case y.kind == infinite:
		return refInf(y.neg), 0
	}
	zeroNeg := mode == ToNegativeInf
	if x.neg == y.neg {
		zeroNeg = x.neg
	}
	return f.sum(mode, zeroNeg, x, y)
}

func (f refFormat) sub(mode RoundingMode, x, y refVal) (refVal, Flags) {
	if y.kind < quietNaN {
		y.neg = !y.neg
	}
	return f.add(mode, x, y)
}

func (f refFormat) mul(mode RoundingMode, x, y refVal) (refVal, Flags) {
	if n, fl, ok := refNaNOf(x, y); ok {
		return n, fl
	}
	neg := x.neg != y.neg
	if x.kind == infinite || y.kind == infinite {
		if x.kind == finite && x.coef.Sign() == 0 || y.kind == finite && y.coef.Sign() == 0 {
			return refNaN, Invalid
		}
		return refInf(neg), 0
	}
	return f.round(mode, neg, new(big.Int).Mul(x.coef, y.coef), x.exp+y.exp, false)
}

func (f refFormat) fma(mode RoundingMode, x, y, z refVal) (refVal, Flags) {
	if x.kind >= quietNaN || y.kind >= quietNaN {
		n, fl, _ := refNaNOf(x, y, z)
		return n, fl
	}
	neg := x.neg != y.neg
	if x.kind == infinite || y.kind == infinite {
		// 0 × Inf is invalid even if z is a NaN.
		if x.kind == finite && x.coef.Sign() == 0 || y.kind == finite && y.coef.Sign() == 0 {
			return refNaN, Invalid
		}
	}
	if n, fl, ok := refNaNOf(z); ok {
		return n, fl
	}
	if x.kind == infinite || y.kind == infinite {
		if z.kind == infinite && z.neg != neg {
			return refNaN, Invalid
		}
		return refInf(neg), 0
	}
	if z.kind == infinite {
		return refInf(z.neg), 0
	}
	p := refVal{neg: neg, coef: new(big.Int).Mul(x.coef, y.coef), exp: x.exp + y.exp}
	zeroNeg := mode == ToNegativeInf
	if p.neg == z.neg {
		zeroNeg = p.neg
	}
	return f.sum(mode, zeroNeg, p, z)
}

// stripZeros removes up to max trailing zeros from coef.
func stripZeros(coef *big.Int, max int) int {
	ten, rem := big.NewInt(10), new(big.Int)
	n := 0
	for ; n < max && coef.Sign() != 0; n++ {
		q := new(big.Int)
		if q.QuoRem(coef, ten, rem); rem.Sign() != 0 {
			break
		}
		coef.Set(q)
	}
	return n
}

func (f refFormat) quo(mode RoundingMode, x, y refVal) (refVal, Flags) {
	if n, fl, ok := refNaNOf(x, y); ok {
		return n, fl
	}
	neg := x.neg != y.neg
	switch {
	case x.kind == infinite && y.kind == infinite:
		return refNaN, Invalid
	case x.kind == infinite:
		return refInf(neg), 0
	case y.kind == infinite:
		return refVal{neg: neg, coef: new(big.Int), exp: f.emin}, 0
	case y.coef.Sign() == 0 && x.coef.Sign() == 0:
		return refNaN, Invalid
	case y.coef.Sign() == 0:
		return refInf(neg), DivisionByZero
	case x.coef.Sign() == 0:
		return f.round(mode, neg, x.coef, x.exp-y.exp, false)
	}
	// Far more digits than the format holds; an exact quotient of p-digit
	// operands never needs this many.
	k := 3*f.prec + 3
	q, rem := new(big.Int).QuoRem(new(big.Int).Mul(x.coef, refPow10(k)), y.coef, new(big.Int))
	exp := x.exp - y.exp - k
	if rem.Sign() == 0 {
		exp += stripZeros(q, k)
	}
	return f.round(mode, neg, q, exp, rem.Sign() != 0)
}

func (f refFormat) sqrt(mode RoundingMode, x refVal) (refVal, Flags) {
	if n, fl, ok := refNaNOf(x); ok {
		return n, fl
	}
	switch {
	case x.kind == finite && x.coef.Sign() == 0:
		return refVal{neg: x.neg, coef: new(big.Int), exp: x.exp >> 1}, 0
	case x.neg:
		return refNaN, Invalid
	case x.kind == infinite:
		return x, 0
	}
	k := 2*f.prec + 6
	if (x.exp-k)&1 != 0 {
		k++
	}
	n := new(big.Int).Mul(x.coef, refPow10(k))
	s := new(big.Int).Sqrt(n)
	exact := new(big.Int).Mul(s, s).Cmp(n) == 0
	exp := (x.exp - k) / 2
	if exact {
		exp += stripZeros(s, x.exp>>1-exp)
	}
	return f.round(mode, false, s, exp, !exact)
}

// random returns a random operand, biased toward the values that break
// arithmetic: short and full-width coefficients, nines, powers of ten,
// halves, exponents at the limits and near each other, and special values.
func (f refFormat) random(r *rand.Rand) refVal {
	v := refVal{neg: r.IntN(2) == 0, coef: new(big.Int)}
	switch r.IntN(40) {
	case 0:
		v.kind = infinite
		return v
	case 1:
		v.kind = quietNaN
		v.coef.SetInt64(r.Int64N(1000))
		return v
	case 2:
		v.kind = signalingNaN
		v.coef.SetInt64(r.Int64N(1000))
		return v
	case 3:
		// zero
	default:
		nd := 1 + r.IntN(f.prec)
		if r.IntN(3) == 0 {
			nd = f.prec
		}
		switch r.IntN(8) {
		case 0:
			v.coef.Sub(refPow10(nd), big.NewInt(int64(1+r.IntN(3)))) // 99..97 to 99..99
		case 1:
			v.coef.Add(refPow10(nd-1), big.NewInt(int64(r.IntN(3)))) // 10..00 to 10..02
		case 2:
			v.coef.Mul(big.NewInt(int64(1+r.IntN(9))), refPow10(nd-1)) // d000..0
		case 3:
			v.coef.Mul(big.NewInt(5), refPow10(nd-1)) // 5000..0
		default:
			for i := 0; i < nd; i++ {
				v.coef.Mul(v.coef, big.NewInt(10)).Add(v.coef, big.NewInt(int64(r.IntN(10))))
			}
		}
	}
	switch r.IntN(10) {
	case 0:
		v.exp = f.emin + r.IntN(2*f.prec)
	case 1:
		v.exp = f.emax - r.IntN(2*f.prec)
	case 2:
		v.exp = f.emin + r.IntN(f.emax-f.emin+1)
	case 3, 4:
		v.exp = r.IntN(4*f.prec) - 2*f.prec
	default:
		v.exp = r.IntN(9) - 6
	}
	return v
}
