package benchmarks

import (
	"testing"

	"github.com/woodsbury/decimal128"
)

var sinkWoodsbury decimal128.Decimal

func benchWoodsbury(b *testing.B, w *workload) {
	var xs [n]decimal128.Decimal
	for i, s := range w.in {
		xs[i] = decimal128.MustParse(s)
	}
	b.Run("Add/woodsbury", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkWoodsbury = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/woodsbury", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkWoodsbury = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/woodsbury", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkWoodsbury = xs[i%n].Quo(xs[(i+1)%n])
		}
	})
	b.Run("Parse/woodsbury", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkWoodsbury, sinkErr = decimal128.Parse(w.in[i%n])
		}
	})
	b.Run("String/woodsbury", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsWoodsbury(w *workload, i int) (results, error) {
	x, err := decimal128.Parse(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := decimal128.MustParse(w.in[(i+1)%n])
	return results{x.Add(y).String(), x.Mul(y).String(), x.Quo(y).String(), x.String()}, nil
}
