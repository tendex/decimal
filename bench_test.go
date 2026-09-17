package decimal

import (
	"math/rand/v2"
	"testing"
)

// The benchmarks run each operation over a fixed set of operand pairs so
// that branch prediction sees a realistic mix rather than a single path.
// "Short" operands are the amounts of everyday commerce, up to nine digits
// with two to four decimal places; "Full" operands use the whole precision.

const benchN = 1024

var (
	sink32  Decimal32
	sink64  Decimal64
	sink128 Decimal128
	sinkStr string
	sinkInt int
)

func benchOperands(full bool, prec int) [benchN]string {
	r := rand.New(rand.NewPCG(42, uint64(prec)))
	var out [benchN]string
	for i := range out {
		nd := 1 + r.IntN(9)
		exp := -2 - r.IntN(3)
		if full {
			nd = prec - r.IntN(2)
			exp = -r.IntN(prec)
		}
		b := make([]byte, 0, 48)
		if r.IntN(4) == 0 {
			b = append(b, '-')
		}
		b = append(b, byte('1'+r.IntN(9)))
		for j := 1; j < nd; j++ {
			b = append(b, byte('0'+r.IntN(10)))
		}
		b = append(b, 'E')
		out[i] = string(b) + itoa(exp)
	}
	return out
}

func operands64(full bool) (xs [benchN]Decimal64) {
	for i, s := range benchOperands(full, 16) {
		xs[i] = MustParse64(s)
	}
	return xs
}

func operands128(full bool) (xs [benchN]Decimal128) {
	for i, s := range benchOperands(full, 34) {
		xs[i] = MustParse128(s)
	}
	return xs
}

func operands32(full bool) (xs [benchN]Decimal32) {
	for i, s := range benchOperands(full, 7) {
		xs[i] = MustParse32(s)
	}
	return xs
}

func bench64(b *testing.B, f func(x, y Decimal64) Decimal64) {
	for _, size := range []struct {
		name string
		full bool
	}{{"Short", false}, {"Full", true}} {
		xs := operands64(size.full)
		b.Run(size.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sink64 = f(xs[i%benchN], xs[(i+1)%benchN])
			}
		})
	}
}

func bench128(b *testing.B, f func(x, y Decimal128) Decimal128) {
	for _, size := range []struct {
		name string
		full bool
	}{{"Short", false}, {"Full", true}} {
		xs := operands128(size.full)
		b.Run(size.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sink128 = f(xs[i%benchN], xs[(i+1)%benchN])
			}
		})
	}
}

func bench32(b *testing.B, f func(x, y Decimal32) Decimal32) {
	xs := operands32(true)
	for i := 0; i < b.N; i++ {
		sink32 = f(xs[i%benchN], xs[(i+1)%benchN])
	}
}

func BenchmarkAdd64(b *testing.B) { bench64(b, Decimal64.Add) }
func BenchmarkSub64(b *testing.B) { bench64(b, Decimal64.Sub) }
func BenchmarkMul64(b *testing.B) { bench64(b, Decimal64.Mul) }
func BenchmarkQuo64(b *testing.B) { bench64(b, Decimal64.Quo) }
func BenchmarkFMA64(b *testing.B) {
	bench64(b, func(x, y Decimal64) Decimal64 { return x.FMA(y, x) })
}
func BenchmarkSqrt64(b *testing.B) {
	bench64(b, func(x, y Decimal64) Decimal64 { return x.Abs().Sqrt() })
}
func BenchmarkRemainder64(b *testing.B) { bench64(b, Decimal64.Remainder) }
func BenchmarkQuantize64(b *testing.B) {
	q := MustParse64("0.01")
	bench64(b, func(x, y Decimal64) Decimal64 { return x.Quantize(q) })
}
func BenchmarkRound64(b *testing.B) {
	bench64(b, func(x, y Decimal64) Decimal64 { return x.Round(1) })
}
func BenchmarkCmp64(b *testing.B) {
	bench64(b, func(x, y Decimal64) Decimal64 { sinkInt = x.Cmp(y); return x })
}

func BenchmarkAdd128(b *testing.B) { bench128(b, Decimal128.Add) }
func BenchmarkSub128(b *testing.B) { bench128(b, Decimal128.Sub) }
func BenchmarkMul128(b *testing.B) { bench128(b, Decimal128.Mul) }
func BenchmarkQuo128(b *testing.B) { bench128(b, Decimal128.Quo) }
func BenchmarkFMA128(b *testing.B) {
	bench128(b, func(x, y Decimal128) Decimal128 { return x.FMA(y, x) })
}
func BenchmarkSqrt128(b *testing.B) {
	bench128(b, func(x, y Decimal128) Decimal128 { return x.Abs().Sqrt() })
}
func BenchmarkCmp128(b *testing.B) {
	bench128(b, func(x, y Decimal128) Decimal128 { sinkInt = x.Cmp(y); return x })
}

func BenchmarkAdd32(b *testing.B) { bench32(b, Decimal32.Add) }
func BenchmarkMul32(b *testing.B) { bench32(b, Decimal32.Mul) }
func BenchmarkQuo32(b *testing.B) { bench32(b, Decimal32.Quo) }

func BenchmarkParse64(b *testing.B) {
	for _, size := range []struct {
		name string
		full bool
	}{{"Short", false}, {"Full", true}} {
		xs := operands64(size.full)
		var ss [benchN]string
		for i, x := range xs {
			ss[i] = x.String()
		}
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sink64, _ = Parse64(ss[i%benchN])
			}
		})
	}
}

func BenchmarkParse128(b *testing.B) {
	xs := operands128(true)
	var ss [benchN]string
	for i, x := range xs {
		ss[i] = x.String()
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink128, _ = Parse128(ss[i%benchN])
	}
}

func BenchmarkString64(b *testing.B) {
	xs := operands64(true)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkStr = xs[i%benchN].String()
	}
}

func BenchmarkAppendText64(b *testing.B) {
	xs := operands64(true)
	buf := make([]byte, 0, 64)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf, _ = xs[i%benchN].AppendText(buf[:0])
	}
	sinkInt = len(buf)
}

func BenchmarkAppendText128(b *testing.B) {
	xs := operands128(true)
	buf := make([]byte, 0, 64)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf, _ = xs[i%benchN].AppendText(buf[:0])
	}
	sinkInt = len(buf)
}

func BenchmarkFloat64From64(b *testing.B) {
	xs := operands64(false)
	var f float64
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f += xs[i%benchN].Float64()
	}
	sinkInt = int(f)
}

func BenchmarkNew64FromFloat(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink64 = New64FromFloat(float64(i%benchN) + 0.1)
	}
}
