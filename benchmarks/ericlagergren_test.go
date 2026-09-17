package benchmarks

import (
	"testing"

	"github.com/ericlagergren/decimal"
)

// ericlagergren/decimal has arbitrary precision; the Context64 and
// Context128 contexts round results to 16 and 34 digits. Results are written
// to a reused destination, as the API intends.

var sinkBig *decimal.Big

func ericContext(digits int) decimal.Context {
	if digits == 34 {
		return decimal.Context128
	}
	return decimal.Context64
}

func benchEricLagergren(b *testing.B, w *workload) {
	ctx := ericContext(w.digits)
	xs := make([]decimal.Big, n)
	for i, s := range w.in {
		xs[i].Context = ctx
		if _, ok := xs[i].SetString(s); !ok {
			b.Fatalf("cannot parse %q", s)
		}
	}
	z := new(decimal.Big)
	z.Context = ctx
	b.Run("Add/ericlagergren", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkBig = z.Add(&xs[i%n], &xs[(i+1)%n])
		}
	})
	b.Run("Mul/ericlagergren", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkBig = z.Mul(&xs[i%n], &xs[(i+1)%n])
		}
	})
	b.Run("Quo/ericlagergren", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkBig = z.Quo(&xs[i%n], &xs[(i+1)%n])
		}
	})
	b.Run("Parse/ericlagergren", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkBig, sinkBool = z.SetString(w.in[i%n])
		}
	})
	b.Run("String/ericlagergren", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsEricLagergren(w *workload, i int) (results, error) {
	ctx := ericContext(w.digits)
	var x, y, add, mul, quo decimal.Big
	for _, d := range []*decimal.Big{&x, &y, &add, &mul, &quo} {
		d.Context = ctx
	}
	x.SetString(w.in[i])
	y.SetString(w.in[(i+1)%n])
	add.Add(&x, &y)
	mul.Mul(&x, &y)
	quo.Quo(&x, &y)
	for _, d := range []*decimal.Big{&x, &y, &add, &mul, &quo} {
		if err := d.Context.Err(); err != nil {
			return results{}, err
		}
	}
	return results{add.String(), mul.String(), quo.String(), x.String()}, nil
}
