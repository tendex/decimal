package decimal

import (
	"encoding/binary"
	"math/big"
	"math/rand/v2"
	"testing"
)

func big128(x uint128) *big.Int {
	z := new(big.Int).SetUint64(x.hi)
	return z.Lsh(z, 64).Or(z, new(big.Int).SetUint64(x.lo))
}

// from256 converts v < 2**256, independently of the platform's word size.
func fromBig(v *big.Int) (x uint256) {
	var b [32]byte
	v.FillBytes(b[:])
	for i := range x {
		x[i] = binary.BigEndian.Uint64(b[24-8*i:])
	}
	return x
}

func big256(x uint256) *big.Int {
	z := new(big.Int)
	for i := 3; i >= 0; i-- {
		z.Lsh(z, 64).Or(z, new(big.Int).SetUint64(x[i]))
	}
	return z
}

// randBits returns a random value with a random bit length up to n, which
// exercises the carry chains and leading-zero paths far better than uniform
// values do.
func randBits(r *rand.Rand, n int) uint256 {
	var x uint256
	for i := range x {
		x[i] = r.Uint64()
	}
	keep := r.IntN(n + 1)
	for i := range x {
		switch lo := 64 * i; {
		case keep <= lo:
			x[i] = 0
		case keep < lo+64:
			x[i] &= 1<<(keep-lo) - 1
		}
	}
	if r.IntN(8) == 0 { // all ones below the top
		for i := range x {
			if x[i] != 0 {
				x[i] = ^uint64(0) >> r.IntN(64)
			}
		}
	}
	return x
}

func TestWideArithmetic(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	mod256 := new(big.Int).Lsh(big.NewInt(1), 256)
	for range 200000 {
		x, y := randBits(r, 256), randBits(r, 256)
		bx, by := big256(x), big256(y)

		if got, want := x.cmp(y), bx.Cmp(by); got != want {
			t.Fatalf("cmp(%x, %x) = %d, want %d", x, y, got, want)
		}
		sum := new(big.Int).Add(bx, by)
		if got := big256(x.add(y)); got.Cmp(sum.Mod(sum, mod256)) != 0 {
			t.Fatalf("%x + %x = %x", x, y, got)
		}
		if x.cmp(y) >= 0 {
			if got := big256(x.sub(y)); got.Cmp(new(big.Int).Sub(bx, by)) != 0 {
				t.Fatalf("%x - %x = %x", x, y, got)
			}
		}
		if got, want := x.bitLen(), bx.BitLen(); got != want {
			t.Fatalf("bitLen(%x) = %d, want %d", x, got, want)
		}

		// 128×128 -> 256 multiplication.
		a, b := randBits(r, 128).low128(), randBits(r, 128).low128()
		if got, want := big256(a.mul(b)), new(big.Int).Mul(big128(a), big128(b)); got.Cmp(want) != 0 {
			t.Fatalf("%x * %x = %x, want %x", a, b, got, want)
		}

		n := uint(r.IntN(256))
		shifted := new(big.Int).Lsh(bx, n)
		if got := big256(x.lsh(n)); got.Cmp(shifted.Mod(shifted, mod256)) != 0 {
			t.Fatalf("%x << %d = %x", x, n, got)
		}

		// Division by a power of ten, through the precomputed reciprocals.
		k := r.IntN(20)
		pow := new(big.Int).SetUint64(pow10tab[k])
		q10, r10 := x.quoRemPow10(k)
		wq10, wr10 := new(big.Int).QuoRem(bx, pow, new(big.Int))
		if big256(q10).Cmp(wq10) != 0 || wr10.Uint64() != r10 {
			t.Fatalf("%x / 1e%d = %x rem %x, want %x rem %x", x, k, q10, r10, wq10, wr10)
		}
		q11, r11 := a.quoRemPow10(k)
		wq10, wr10 = wq10.QuoRem(big128(a), pow, wr10)
		if big128(q11).Cmp(wq10) != 0 || wr10.Uint64() != r11 {
			t.Fatalf("%x / 1e%d = %x rem %x, want %x rem %x", a, k, q11, r11, wq10, wr10)
		}

		// Division by one and two words.
		if d := y[0]; d != 0 {
			q, rem := x.quoRem64(d)
			wq, wr := new(big.Int).QuoRem(bx, new(big.Int).SetUint64(d), new(big.Int))
			if big256(q).Cmp(wq) != 0 || wr.Uint64() != rem {
				t.Fatalf("%x / %x = %x rem %x, want %x rem %x", x, d, q, rem, wq, wr)
			}
		}
		if d := b; !d.isZero() {
			q, rem := x.quoRem128(d)
			wq, wr := new(big.Int).QuoRem(bx, big128(d), new(big.Int))
			if big256(q).Cmp(wq) != 0 || big128(rem).Cmp(wr) != 0 {
				t.Fatalf("%x / %x = %x rem %x, want %x rem %x", x, d, q, rem, wq, wr)
			}
			q1, r1 := a.quoRem(d)
			wq, wr = new(big.Int).QuoRem(big128(a), big128(d), new(big.Int))
			if big128(q1).Cmp(wq) != 0 || big128(r1).Cmp(wr) != 0 {
				t.Fatalf("%x / %x = %x rem %x, want %x rem %x", a, d, q1, r1, wq, wr)
			}
		}
	}
}

func TestDigitCounts(t *testing.T) {
	ten := big.NewInt(10)
	p := big.NewInt(1)
	for n := 1; n <= 77; n++ {
		// p = 10**(n-1), the smallest n-digit number.
		for delta := -1; delta <= 1; delta++ {
			v := new(big.Int).Add(p, big.NewInt(int64(delta)))
			want := len(v.String())
			if v.Sign() == 0 {
				want = 0
			}
			x := fromBig(v)
			if got := x.ndigits(); got != want {
				t.Errorf("ndigits(%s) = %d, want %d", v, got, want)
			}
			if v.BitLen() <= 128 {
				if got := x.low128().ndigits(); got != want {
					t.Errorf("uint128 ndigits(%s) = %d, want %d", v, got, want)
				}
			}
			if v.IsUint64() {
				if got := ndigits64(v.Uint64()); got != want {
					t.Errorf("ndigits64(%s) = %d, want %d", v, got, want)
				}
			}
		}
		if n < 77 {
			if got := big256(pow10tab256[n]); got.Cmp(new(big.Int).Mul(p, ten)) != 0 {
				t.Errorf("pow10tab256[%d] = %s", n, got)
			}
			if got := big256(pow10tab256[n-1].mulPow10(1)); got.Cmp(new(big.Int).Mul(p, ten)) != 0 {
				t.Errorf("mulPow10: 10**%d = %s", n, got)
			}
		}
		p.Mul(p, ten)
	}
	// Every bit length, so that the log10(2) approximation is covered.
	for n := 1; n <= 256; n++ {
		for _, v := range []*big.Int{
			new(big.Int).Lsh(big.NewInt(1), uint(n-1)),
			new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(n)), big.NewInt(1)),
		} {
			x := fromBig(v)
			if got, want := x.ndigits(), len(v.String()); got != want {
				t.Errorf("ndigits(%s) = %d, want %d", v, got, want)
			}
		}
	}
}

func TestShiftRight(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for range 100000 {
		x := randBits(r, 256)
		bx := big256(x)
		nd := x.ndigits()
		// The quotient must fit in 128 bits.
		drop := max(nd-38, 1) + r.IntN(45)
		pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(drop)), nil)
		wq, wr := new(big.Int).QuoRem(bx, pow, new(big.Int))
		want := remZero
		if wr.Sign() != 0 {
			want = remainder(2 + wr.Lsh(wr, 1).Cmp(pow))
		}
		q, rem := shiftRight256(x, drop)
		if big128(q).Cmp(wq) != 0 || rem != want {
			t.Fatalf("shiftRight256(%s, %d) = %s, %d; want %s, %d", bx, drop, big128(q), rem, wq, want)
		}
		if x.fits128() && wq.IsUint64() {
			q, rem := shiftRight(x.low128(), drop)
			if q != wq.Uint64() || rem != want {
				t.Fatalf("shiftRight(%s, %d) = %d, %d; want %s, %d", bx, drop, q, rem, wq, want)
			}
		}
	}
}
