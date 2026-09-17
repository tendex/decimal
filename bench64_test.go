package decimal

import "testing"

var sink64 Decimal64

func operands64(full bool) (xs [benchN]Decimal64) {
	for i, s := range benchOperands(full, digits64) {
		xs[i] = MustParse64(s)
	}
	return xs
}

// TestBenchOperands64 checks that every benchmarked operation succeeds on the
// benchmark operands, so that no benchmark measures an error path.
func TestBenchOperands64(t *testing.T) {
	cent := MustParse64("0.01")
	for _, xs := range [...][benchN]Decimal64{operands64(false), operands64(true)} {
		for i := range xs {
			x, y, z := xs[i], xs[(i+1)%benchN], xs[(i+2)%benchN]
			for _, r := range []Decimal64{
				x.Add(y), x.Sub(y), x.Mul(y), x.Quo(y), x.FMA(y, z), x.Abs().Sqrt(),
				x.Remainder(y), x.Quantize(cent), x.Round(2), New64FromFloat(x.Float64()),
			} {
				if !r.IsFinite() {
					t.Fatalf("operation on %v, %v, %v gives %v", x, y, z, r)
				}
			}
			if _, err := Parse64(x.String()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func BenchmarkDecimal64(b *testing.B) {
	short, full := operands64(false), operands64(true)
	each := func(op string, f func(b *testing.B, xs *[benchN]Decimal64)) {
		b.Run(op+"/Short", func(b *testing.B) { f(b, &short) })
		b.Run(op+"/Full", func(b *testing.B) { f(b, &full) })
	}
	cent := MustParse64("0.01")

	each("Add", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Add(xs[(i+1)%benchN])
		}
	})
	each("Sub", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Sub(xs[(i+1)%benchN])
		}
	})
	each("Mul", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Mul(xs[(i+1)%benchN])
		}
	})
	each("Quo", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Quo(xs[(i+1)%benchN])
		}
	})
	each("FMA", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].FMA(xs[(i+1)%benchN], xs[(i+2)%benchN])
		}
	})
	each("Sqrt", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Abs().Sqrt()
		}
	})
	each("Remainder", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Remainder(xs[(i+1)%benchN])
		}
	})
	each("Quantize", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Quantize(cent)
		}
	})
	each("Round", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sink64 = xs[i%benchN].Round(2)
		}
	})
	each("Cmp", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sinkInt = xs[i%benchN].Cmp(xs[(i+1)%benchN])
		}
	})
	each("Parse", func(b *testing.B, xs *[benchN]Decimal64) {
		var ss [benchN]string
		for i, x := range xs {
			ss[i] = x.String()
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sink64, _ = Parse64(ss[i%benchN])
		}
	})
	each("String", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sinkStr = xs[i%benchN].String()
		}
	})
	each("AppendText", func(b *testing.B, xs *[benchN]Decimal64) {
		buf := make([]byte, 0, 64)
		for i := 0; i < b.N; i++ {
			buf, _ = xs[i%benchN].AppendText(buf[:0])
		}
		sinkBuf = buf
	})
	each("Int64", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sinkInt64, sinkBool = xs[i%benchN].Int64()
		}
	})
	each("Float64", func(b *testing.B, xs *[benchN]Decimal64) {
		for i := 0; i < b.N; i++ {
			sinkFloat = xs[i%benchN].Float64()
		}
	})
	each("FromFloat", func(b *testing.B, xs *[benchN]Decimal64) {
		var fs [benchN]float64
		for i, x := range xs {
			fs[i] = x.Float64()
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sink64 = New64FromFloat(fs[i%benchN])
		}
	})
}
