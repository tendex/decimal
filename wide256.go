package decimal

import "math/bits"

// The uint256 type is declared in wide.go: four words, least significant
// first. It holds the intermediate results of the 128-bit kernel (products
// and scaled dividends of two 34-digit coefficients) and of fused
// multiply-add in every format. 2**256 exceeds 10**77.

func u256(x uint128) uint256 { return uint256{x.lo, x.hi} }

func (x uint256) isZero() bool { return x[0]|x[1]|x[2]|x[3] == 0 }

// fits128 reports whether x < 2**128.
func (x uint256) fits128() bool { return x[2]|x[3] == 0 }

// low128 returns x mod 2**128.
func (x uint256) low128() uint128 { return uint128{x[1], x[0]} }

// cmp returns -1, 0 or +1 as x is less than, equal to or greater than y.
func (x uint256) cmp(y uint256) int {
	for i := 3; i >= 0; i-- {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func (x uint256) add(y uint256) (z uint256) {
	var c uint64
	z[0], c = bits.Add64(x[0], y[0], 0)
	z[1], c = bits.Add64(x[1], y[1], c)
	z[2], c = bits.Add64(x[2], y[2], c)
	z[3], _ = bits.Add64(x[3], y[3], c)
	return z
}

func (x uint256) sub(y uint256) (z uint256) {
	var b uint64
	z[0], b = bits.Sub64(x[0], y[0], 0)
	z[1], b = bits.Sub64(x[1], y[1], b)
	z[2], b = bits.Sub64(x[2], y[2], b)
	z[3], _ = bits.Sub64(x[3], y[3], b)
	return z
}

func (x uint256) sub64(y uint64) uint256 { return x.sub(uint256{y}) }

// lsh returns x<<n for n < 256.
func (x uint256) lsh(n uint) (z uint256) {
	words, shift := int(n/64), n%64
	for i := 3; i >= words; i-- {
		z[i] = x[i-words] << shift
		if shift != 0 && i > words {
			z[i] |= x[i-words-1] >> (64 - shift)
		}
	}
	return z
}

// mul64 returns x*y. The caller guarantees the product fits in 256 bits.
func (x uint256) mul64(y uint64) (z uint256) {
	var carry uint64
	for i := range x {
		hi, lo := bits.Mul64(x[i], y)
		var c uint64
		z[i], c = bits.Add64(lo, carry, 0)
		carry = hi + c
	}
	return z
}

// mulPow10 returns x × 10**n. The caller guarantees the product fits.
func (x uint256) mulPow10(n int) uint256 {
	if x.fits128() {
		// One 128×128 multiplication covers the first 38 digits.
		k := min(n, 38)
		x = x.low128().mul(pow10tab128[k])
		n -= k
	}
	for ; n > 19; n -= 19 {
		x = x.mul64(1e19)
	}
	if n > 0 {
		x = x.mul64(pow10tab[n])
	}
	return x
}

// quoRem64 returns x/d and x%d for d != 0.
func (x uint256) quoRem64(d uint64) (q uint256, r uint64) {
	for i := 3; i >= 0; i-- {
		if r == 0 && x[i] < d {
			// Also the common case of leading zero words.
			q[i], r = 0, x[i]
			continue
		}
		q[i], r = bits.Div64(r, x[i], d)
	}
	return q, r
}

// quoRemPow10 returns x / 10**k and x % 10**k for 0 <= k <= 19.
func (x uint256) quoRemPow10(k int) (q uint256, r uint64) {
	d, dv := pow10tab[k], &pow10div[k]
	for i := 3; i >= 0; i-- {
		switch {
		case r == 0 && x[i] < d:
			// Also the common case of leading zero words.
			r = x[i]
		case r == 0:
			q[i], r = x[i]/d, x[i]%d
		default:
			q[i], r = dv.quoRem(r, x[i])
		}
	}
	return q, r
}

// quoRem128 returns x/d and x%d for d != 0 (Knuth, TAOCP 4.3.1, algorithm D
// with a two-word divisor).
func (x uint256) quoRem128(d uint128) (q uint256, r uint128) {
	if d.hi == 0 {
		q, r64 := x.quoRem64(d.lo)
		return q, uint128{0, r64}
	}
	// Normalize so that the divisor's top bit is set.
	s := uint(bits.LeadingZeros64(d.hi))
	v := d.lsh(s)
	u := [5]uint64{x[0], x[1], x[2], x[3], 0}
	if s != 0 {
		u[4] = x[3] >> (64 - s)
		u[3] = x[3]<<s | x[2]>>(64-s)
		u[2] = x[2]<<s | x[1]>>(64-s)
		u[1] = x[1]<<s | x[0]>>(64-s)
		u[0] = x[0] << s
	}
	for j := 2; j >= 0; j-- {
		// Estimate the quotient word from the top two words of the
		// dividend and the top word of the divisor, then correct it using
		// the divisor's second word. The estimate is at most two too big.
		var qhat, rhat uint64
		refine := true
		if u[j+2] >= v.hi {
			qhat = ^uint64(0)
			var c uint64
			rhat, c = bits.Add64(u[j+1], v.hi, 0)
			refine = c == 0
		} else {
			qhat, rhat = bits.Div64(u[j+2], u[j+1], v.hi)
		}
		for refine {
			ph, pl := bits.Mul64(qhat, v.lo)
			if ph < rhat || ph == rhat && pl <= u[j] {
				break
			}
			qhat--
			var c uint64
			rhat, c = bits.Add64(rhat, v.hi, 0)
			refine = c == 0
		}
		// Multiply and subtract; add back in the rare case that the
		// estimate was still one too big.
		ph0, pl0 := bits.Mul64(qhat, v.lo)
		ph1, pl1 := bits.Mul64(qhat, v.hi)
		p1, c := bits.Add64(ph0, pl1, 0)
		p2 := ph1 + c
		var b uint64
		u[j], b = bits.Sub64(u[j], pl0, 0)
		u[j+1], b = bits.Sub64(u[j+1], p1, b)
		u[j+2], b = bits.Sub64(u[j+2], p2, b)
		if b != 0 {
			qhat--
			u[j], c = bits.Add64(u[j], v.lo, 0)
			u[j+1], c = bits.Add64(u[j+1], v.hi, c)
			u[j+2] += c
		}
		q[j] = qhat
	}
	return q, uint128{u[1], u[0]}.rsh(s)
}

// bitLen returns the number of bits needed to represent x.
func (x uint256) bitLen() int {
	for i := 3; i >= 0; i-- {
		if x[i] != 0 {
			return 64*i + bits.Len64(x[i])
		}
	}
	return 0
}

// ndigits returns the number of decimal digits in x; zero has no digits.
func (x uint256) ndigits() int {
	if x.fits128() {
		return x.low128().ndigits()
	}
	t := x.bitLen() * 1233 >> 12
	if x.cmp(pow10tab256[t]) >= 0 {
		t++
	}
	return t
}

// shiftRight256 returns coef / 10**drop, which the caller guarantees fits in
// 128 bits, and a classification of the remainder. drop must be positive.
func shiftRight256(coef uint256, drop int) (uint128, remainder) {
	q, rem := coef.shiftRight(drop)
	return q.low128(), rem
}

// shiftRight returns coef / 10**drop and a classification of the remainder.
// drop must be positive.
func (coef uint256) shiftRight(drop int) (uint256, remainder) {
	switch {
	case coef.isZero():
		return uint256{}, remZero
	case drop > 77:
		// coef < 2**256 < 5×10**77: everything goes, and it is below half.
		return uint256{}, remBelow
	}
	// Divide by 10**19 while more than 19 digits remain to be dropped; the
	// last division decides where the remainder stands relative to half.
	lower := false
	for ; drop > 19; drop -= 19 {
		var r uint64
		coef, r = coef.quoRemPow10(19)
		lower = lower || r != 0
	}
	d := pow10tab[drop]
	q, r := coef.quoRemPow10(drop)
	rem := classify(r, d/2)
	if lower {
		rem = rem.sticky()
	}
	return q, rem
}
