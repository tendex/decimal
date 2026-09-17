package benchmarks

import (
	"testing"

	"github.com/tendex/decimal"
)

var (
	sinkTendex64  decimal.Decimal64
	sinkTendex128 decimal.Decimal128
)

func benchTendex64(b *testing.B, w *workload) {
	var xs [n]decimal.Decimal64
	for i, s := range w.in {
		xs[i] = decimal.MustParse64(s)
	}
	b.Run("Add/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex64 = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex64 = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex64 = xs[i%n].Quo(xs[(i+1)%n])
		}
	})
	b.Run("Parse/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex64, sinkErr = decimal.Parse64(w.in[i%n])
		}
	})
	b.Run("String/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsTendex64(w *workload, i int) (results, error) {
	x, err := decimal.Parse64(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := decimal.MustParse64(w.in[(i+1)%n])
	return results{x.Add(y).String(), x.Mul(y).String(), x.Quo(y).String(), x.String()}, nil
}

func benchTendex128(b *testing.B, w *workload) {
	var xs [n]decimal.Decimal128
	for i, s := range w.in {
		xs[i] = decimal.MustParse128(s)
	}
	b.Run("Add/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex128 = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex128 = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex128 = xs[i%n].Quo(xs[(i+1)%n])
		}
	})
	b.Run("Parse/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTendex128, sinkErr = decimal.Parse128(w.in[i%n])
		}
	})
	b.Run("String/tendex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsTendex128(w *workload, i int) (results, error) {
	x, err := decimal.Parse128(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := decimal.MustParse128(w.in[(i+1)%n])
	return results{x.Add(y).String(), x.Mul(y).String(), x.Quo(y).String(), x.String()}, nil
}
