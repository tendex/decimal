package decimal

import (
	"math/big"
	"math/rand/v2"
	"testing"
)

// randFormat adapts one decimal type to the reference implementation.
type randFormat[T any] struct {
	ref     refFormat
	toRef   func(T) refVal
	fromRef func(refVal) T

	add, sub, mul, quo func(*Context, T, T) T
	fma                func(*Context, T, T, T) T
	sqrt               func(*Context, T) T
}

// runRandom checks the arithmetic operations against the reference
// implementation on n random operand triples in every rounding mode.
func runRandom[T any](t *testing.T, rf randFormat[T], n int) {
	r := rand.New(rand.NewPCG(uint64(rf.ref.prec), 754))
	f := rf.ref
	check := func(op string, mode RoundingMode, got T, gotFlags Flags, want refVal, wantFlags Flags, args ...refVal) {
		t.Helper()
		if g := rf.toRef(got); !g.same(want) || gotFlags != wantFlags {
			t.Fatalf("%s %s %v (%v)\n\t got %v [%v]\n\twant %v [%v]", f.name, op, args, mode, g, gotFlags, want, wantFlags)
		}
	}
	for i := 0; i < n; i++ {
		mode := RoundingMode(r.IntN(5))
		rx, ry, rz := f.random(r), f.random(r), f.random(r)
		if r.IntN(4) == 0 && ry.kind == finite && rx.kind == finite {
			// Nearly equal magnitudes, for cancellation.
			ry.coef, ry.exp = new(big.Int).Set(rx.coef), rx.exp
			if r.IntN(2) == 0 && ry.coef.Sign() != 0 {
				ry.coef.Sub(ry.coef, big.NewInt(1))
			}
		}
		x, y, z := rf.fromRef(rx), rf.fromRef(ry), rf.fromRef(rz)

		c := Context{Rounding: mode}
		got := rf.add(&c, x, y)
		want, wantFlags := f.add(mode, rx, ry)
		check("add", mode, got, c.Flags, want, wantFlags, rx, ry)

		c = Context{Rounding: mode}
		got = rf.sub(&c, x, y)
		want, wantFlags = f.sub(mode, rx, ry)
		check("sub", mode, got, c.Flags, want, wantFlags, rx, ry)

		c = Context{Rounding: mode}
		got = rf.mul(&c, x, y)
		want, wantFlags = f.mul(mode, rx, ry)
		check("mul", mode, got, c.Flags, want, wantFlags, rx, ry)

		c = Context{Rounding: mode}
		got = rf.quo(&c, x, y)
		want, wantFlags = f.quo(mode, rx, ry)
		check("quo", mode, got, c.Flags, want, wantFlags, rx, ry)

		c = Context{Rounding: mode}
		got = rf.fma(&c, x, y, z)
		want, wantFlags = f.fma(mode, rx, ry, rz)
		check("fma", mode, got, c.Flags, want, wantFlags, rx, ry, rz)

		c = Context{Rounding: mode}
		got = rf.sqrt(&c, x)
		want, wantFlags = f.sqrt(mode, rx)
		check("sqrt", mode, got, c.Flags, want, wantFlags, rx)
	}
}

var rand64 = randFormat[Decimal64]{
	ref: ref64,
	toRef: func(x Decimal64) refVal {
		n := x.unpack()
		return refVal{kind: n.kind, neg: n.neg, coef: new(big.Int).SetUint64(n.coef), exp: int(n.exp)}
	},
	fromRef: func(v refVal) Decimal64 {
		return pack64(num{coef: v.coef.Uint64(), exp: int32(v.exp), neg: v.neg, kind: v.kind})
	},
	add:  (*Context).Add64,
	sub:  (*Context).Sub64,
	mul:  (*Context).Mul64,
	quo:  (*Context).Quo64,
	fma:  (*Context).FMA64,
	sqrt: (*Context).Sqrt64,
}

var rand32 = randFormat[Decimal32]{
	ref: ref32,
	toRef: func(x Decimal32) refVal {
		n := x.unpack()
		return refVal{kind: n.kind, neg: n.neg, coef: new(big.Int).SetUint64(n.coef), exp: int(n.exp)}
	},
	fromRef: func(v refVal) Decimal32 {
		return pack32(num{coef: v.coef.Uint64(), exp: int32(v.exp), neg: v.neg, kind: v.kind})
	},
	add:  (*Context).Add32,
	sub:  (*Context).Sub32,
	mul:  (*Context).Mul32,
	quo:  (*Context).Quo32,
	fma:  (*Context).FMA32,
	sqrt: (*Context).Sqrt32,
}

func TestRandom32(t *testing.T) { runRandom(t, rand32, randomN()) }

var rand128 = randFormat[Decimal128]{
	ref: ref128,
	toRef: func(x Decimal128) refVal {
		n := x.unpack()
		return refVal{kind: n.kind, neg: n.neg, coef: big128(n.coef), exp: int(n.exp)}
	},
	fromRef: func(v refVal) Decimal128 {
		var w [2]uint64
		for i, word := range v.coef.Bits() {
			w[i] = uint64(word)
		}
		return pack128(num128{coef: uint128{w[1], w[0]}, exp: int32(v.exp), neg: v.neg, kind: v.kind})
	},
	add:  (*Context).Add128,
	sub:  (*Context).Sub128,
	mul:  (*Context).Mul128,
	quo:  (*Context).Quo128,
	fma:  (*Context).FMA128,
	sqrt: (*Context).Sqrt128,
}

func TestRandom128(t *testing.T) { runRandom(t, rand128, randomN()) }

func randomN() int {
	if testing.Short() {
		return 20000
	}
	return 300000
}

func TestRandom64(t *testing.T) { runRandom(t, rand64, randomN()) }
