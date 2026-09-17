package benchmarks

import (
	"testing"

	anz "github.com/anz-bank/decimal"
)

var sinkANZ anz.Decimal64

func benchANZ(b *testing.B, w *workload) {
	var xs [n]anz.Decimal64
	for i, s := range w.in {
		xs[i] = anz.MustParse64(s)
	}
	b.Run("Add/anz", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkANZ = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/anz", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkANZ = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/anz", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkANZ = xs[i%n].Quo(xs[(i+1)%n])
		}
	})
	b.Run("Parse/anz", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkANZ, sinkErr = anz.Parse64(w.in[i%n])
		}
	})
	b.Run("String/anz", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsANZ(w *workload, i int) (results, error) {
	x, err := anz.Parse64(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := anz.MustParse64(w.in[(i+1)%n])
	return results{x.Add(y).String(), x.Mul(y).String(), x.Quo(y).String(), x.String()}, nil
}
