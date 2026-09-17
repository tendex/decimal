package benchmarks

import (
	"strconv"
	"testing"
)

// float64 is not a decimal type: it cannot represent most of the operands
// exactly. It is included to show what exact decimal arithmetic costs.

var sinkFloat64 float64

func benchFloat64(b *testing.B, w *workload) {
	var xs [n]float64
	for i, s := range w.in {
		xs[i], _ = strconv.ParseFloat(s, 64)
	}
	b.Run("Add/float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkFloat64 = xs[i%n] + xs[(i+1)%n]
		}
	})
	b.Run("Mul/float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkFloat64 = xs[i%n] * xs[(i+1)%n]
		}
	})
	b.Run("Quo/float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkFloat64 = xs[i%n] / xs[(i+1)%n]
		}
	})
	b.Run("Parse/float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkFloat64, sinkErr = strconv.ParseFloat(w.in[i%n], 64)
		}
	})
	b.Run("String/float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = strconv.FormatFloat(xs[i%n], 'g', -1, 64)
		}
	})
}

func resultsFloat64(w *workload, i int) (results, error) {
	x, err := strconv.ParseFloat(w.in[i], 64)
	if err != nil {
		return results{}, err
	}
	y, _ := strconv.ParseFloat(w.in[(i+1)%n], 64)
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	return results{f(x + y), f(x * y), f(x / y), f(x)}, nil
}
