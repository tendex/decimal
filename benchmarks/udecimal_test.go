package benchmarks

import (
	"testing"

	"github.com/quagmt/udecimal"
)

var sinkUdecimal udecimal.Decimal

func benchUdecimal(b *testing.B, w *workload) {
	var xs [n]udecimal.Decimal
	for i, s := range w.in {
		xs[i] = udecimal.MustParse(s)
	}
	b.Run("Add/udecimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkUdecimal = xs[i%n].Add(xs[(i+1)%n])
		}
	})
	b.Run("Mul/udecimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkUdecimal = xs[i%n].Mul(xs[(i+1)%n])
		}
	})
	b.Run("Quo/udecimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkUdecimal, sinkErr = xs[i%n].Div(xs[(i+1)%n])
		}
	})
	b.Run("Parse/udecimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkUdecimal, sinkErr = udecimal.Parse(w.in[i%n])
		}
	})
	b.Run("String/udecimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsUdecimal(w *workload, i int) (results, error) {
	x, err := udecimal.Parse(w.in[i])
	if err != nil {
		return results{}, err
	}
	y := udecimal.MustParse(w.in[(i+1)%n])
	quo, err := x.Div(y)
	if err != nil {
		return results{}, err
	}
	return results{x.Add(y).String(), x.Mul(y).String(), quo.String(), x.String()}, nil
}
