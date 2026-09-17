package decimal

import "math/bits"

//go:generate go run gen_tables.go

// uint128 is an unsigned 128-bit integer.
type uint128 struct{ hi, lo uint64 }

// uint256 is an unsigned 256-bit integer, least significant word first.
type uint256 [4]uint64

func (x uint128) isZero() bool { return x.hi|x.lo == 0 }

// less reports whether x < y.
func (x uint128) less(y uint128) bool {
	return x.hi < y.hi || x.hi == y.hi && x.lo < y.lo
}

// cmp returns -1, 0 or +1 as x is less than, equal to or greater than y.
func (x uint128) cmp(y uint128) int {
	switch {
	case x == y:
		return 0
	case x.less(y):
		return -1
	}
	return 1
}

func (x uint128) add(y uint128) uint128 {
	lo, c := bits.Add64(x.lo, y.lo, 0)
	hi, _ := bits.Add64(x.hi, y.hi, c)
	return uint128{hi, lo}
}

func (x uint128) add64(y uint64) uint128 {
	lo, c := bits.Add64(x.lo, y, 0)
	return uint128{x.hi + c, lo}
}

func (x uint128) sub(y uint128) uint128 {
	lo, b := bits.Sub64(x.lo, y.lo, 0)
	hi, _ := bits.Sub64(x.hi, y.hi, b)
	return uint128{hi, lo}
}

func (x uint128) sub64(y uint64) uint128 {
	lo, b := bits.Sub64(x.lo, y, 0)
	return uint128{x.hi - b, lo}
}

// lsh returns x<<n for n < 128.
func (x uint128) lsh(n uint) uint128 {
	if n >= 64 {
		return uint128{x.lo << (n - 64), 0}
	}
	return uint128{x.hi<<n | x.lo>>(64-n), x.lo << n}
}

// rsh returns x>>n for n < 128.
func (x uint128) rsh(n uint) uint128 {
	if n >= 64 {
		return uint128{0, x.hi >> (n - 64)}
	}
	return uint128{x.hi >> n, x.lo>>n | x.hi<<(64-n)}
}

// mul64 returns x*y. The caller guarantees the product fits in 128 bits.
func (x uint128) mul64(y uint64) uint128 {
	hi, lo := bits.Mul64(x.lo, y)
	return uint128{hi + x.hi*y, lo}
}

// mul returns the full 256-bit product x*y.
func (x uint128) mul(y uint128) uint256 {
	var z uint256
	var c uint64
	h00, l00 := bits.Mul64(x.lo, y.lo)
	h01, l01 := bits.Mul64(x.lo, y.hi)
	h10, l10 := bits.Mul64(x.hi, y.lo)
	h11, l11 := bits.Mul64(x.hi, y.hi)
	z[0] = l00
	z[1], c = bits.Add64(h00, l01, 0)
	z[2], c = bits.Add64(h01, l11, c)
	z[3] = h11 + c
	z[1], c = bits.Add64(z[1], l10, 0)
	z[2], c = bits.Add64(z[2], h10, c)
	z[3] += c
	return z
}

// quoRem64 returns x/d and x%d for d != 0.
func (x uint128) quoRem64(d uint64) (q uint128, r uint64) {
	if x.hi == 0 {
		return uint128{0, x.lo / d}, x.lo % d
	}
	q.hi, r = x.hi/d, x.hi%d
	q.lo, r = bits.Div64(r, x.lo, d)
	return q, r
}

// quoRem returns x/d and x%d for d != 0.
func (x uint128) quoRem(d uint128) (q, r uint128) {
	if d.hi == 0 {
		q, r64 := x.quoRem64(d.lo)
		return q, uint128{0, r64}
	}
	// The quotient fits in 64 bits. Estimate it from the top 64 bits of
	// the normalized divisor (Hacker's Delight, 9-5); after the decrement
	// the estimate is either exact or one too small.
	n := uint(bits.LeadingZeros64(d.hi))
	v1 := d.lsh(n).hi
	u1 := x.rsh(1)
	tq, _ := bits.Div64(u1.hi, u1.lo, v1)
	tq >>= 63 - n
	if tq != 0 {
		tq--
	}
	r = x.sub(d.mul64(tq))
	if !r.less(d) {
		tq++
		r = r.sub(d)
	}
	return uint128{0, tq}, r
}

// ndigits64 returns the number of decimal digits in x; ndigits64(0) is 0.
func ndigits64(x uint64) int {
	// bits.Len64(x)*log10(2) is the digit count or one less.
	t := bits.Len64(x) * 1233 >> 12
	if t < len(pow10tab) && x >= pow10tab[t] {
		t++
	}
	return t
}

// ndigits returns the number of decimal digits in x; zero has no digits.
func (x uint128) ndigits() int {
	if x.hi == 0 {
		return ndigits64(x.lo)
	}
	t := (64 + bits.Len64(x.hi)) * 1233 >> 12
	if !x.less(pow10tab128[t]) {
		t++
	}
	return t
}

// trailingZeros64 returns the largest n <= max such that 10**n divides x,
// together with x / 10**n. x must be non-zero.
func trailingZeros64(x uint64, max int) (uint64, int) {
	n := 0
	for n+8 <= max && x%1e8 == 0 {
		x /= 1e8
		n += 8
	}
	if n+4 <= max && x%1e4 == 0 {
		x /= 1e4
		n += 4
	}
	if n+2 <= max && x%100 == 0 {
		x /= 100
		n += 2
	}
	if n+1 <= max && x%10 == 0 {
		x /= 10
		n++
	}
	// A second pass of the small steps covers runs such as 7 = 4+2+1 that
	// the descending sequence above cannot reach after a partial match.
	for n < max && x%10 == 0 {
		x /= 10
		n++
	}
	return x, n
}
