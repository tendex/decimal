package benchmarks

import (
	"testing"

	govalues "github.com/govalues/decimal"
)

var sinkGovalues govalues.Decimal

func benchGovalues(b *testing.B, w *workload) {
	var xs [n]govalues.Decimal
	for i, s := range w.in {
		xs[i] = govalues.MustParse(s)
	}
	b.Run("Add/govalues", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkGovalues, sinkErr = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/govalues", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkGovalues, sinkErr = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/govalues", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkGovalues, sinkErr = xs[i%n].Quo(xs[(i+1)%n])
		}
	})
	b.Run("Parse/govalues", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkGovalues, sinkErr = govalues.Parse(w.in[i%n])
		}
	})
	b.Run("String/govalues", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsGovalues(w *workload, i int) (results, error) {
	x, err := govalues.Parse(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := govalues.MustParse(w.in[(i+1)%n])
	add, err1 := x.Add(y)
	mul, err2 := x.Mul(y)
	quo, err3 := x.Quo(y)
	for _, err := range []error{err1, err2, err3} {
		if err != nil {
			return results{}, err
		}
	}
	return results{add.String(), mul.String(), quo.String(), x.String()}, nil
}
