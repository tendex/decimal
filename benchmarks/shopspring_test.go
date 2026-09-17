package benchmarks

import (
	"testing"

	"github.com/shopspring/decimal"
)

// shopspring/decimal has arbitrary precision: sums and products are exact,
// and quotients are rounded to a number of decimal places, here the
// precision of the workload.

var sinkShopspring decimal.Decimal

func benchShopspring(b *testing.B, w *workload) {
	var xs [n]decimal.Decimal
	for i, s := range w.in {
		xs[i] = decimal.RequireFromString(s)
	}
	places := int32(w.digits)
	b.Run("Add/shopspring", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkShopspring = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/shopspring", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkShopspring = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/shopspring", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkShopspring = xs[i%n].DivRound(xs[(i+1)%n], places)
		}
	})
	b.Run("Parse/shopspring", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkShopspring, sinkErr = decimal.NewFromString(w.in[i%n])
		}
	})
	b.Run("String/shopspring", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsShopspring(w *workload, i int) (results, error) {
	x, err := decimal.NewFromString(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := decimal.RequireFromString(w.in[(i+1)%n])
	quo := x.DivRound(y, int32(w.digits))
	return results{x.Add(y).String(), x.Mul(y).String(), quo.String(), x.String()}, nil
}
