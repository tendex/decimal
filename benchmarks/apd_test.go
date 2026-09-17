package benchmarks

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
)

// cockroachdb/apd has arbitrary precision; the context rounds results to the
// precision of the workload. Results are written to a reused destination, as
// the API intends.

var sinkCondition apd.Condition

func apdContext(digits int) *apd.Context {
	c := apd.BaseContext.WithPrecision(uint32(digits))
	c.Rounding = apd.RoundHalfEven
	return c
}

func benchAPD(b *testing.B, w *workload) {
	ctx := apdContext(w.digits)
	xs := make([]apd.Decimal, n)
	for i, s := range w.in {
		if _, _, err := xs[i].SetString(s); err != nil {
			b.Fatal(err)
		}
	}
	var z apd.Decimal
	b.Run("Add/apd", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkCondition, sinkErr = ctx.Add(&z, &xs[i%n], &xs[(i+1)%n])
		}
	})
	b.Run("Mul/apd", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkCondition, sinkErr = ctx.Mul(&z, &xs[i%n], &xs[(i+1)%n])
		}
	})
	b.Run("Quo/apd", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkCondition, sinkErr = ctx.Quo(&z, &xs[i%n], &xs[(i+1)%n])
		}
	})
	b.Run("Parse/apd", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, sinkCondition, sinkErr = z.SetString(w.in[i%n])
		}
	})
	b.Run("String/apd", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkString = xs[i%n].String()
		}
	})
}

func resultsAPD(w *workload, i int) (results, error) {
	ctx := apdContext(w.digits)
	var x, y, add, mul, quo apd.Decimal
	if _, _, err := x.SetString(w.in[i]); err != nil {
		return results{}, err
	}
	if _, _, err := y.SetString(w.in[(i+1)%n]); err != nil {
		return results{}, err
	}
	for _, op := range []func() (apd.Condition, error){
		func() (apd.Condition, error) { return ctx.Add(&add, &x, &y) },
		func() (apd.Condition, error) { return ctx.Mul(&mul, &x, &y) },
		func() (apd.Condition, error) { return ctx.Quo(&quo, &x, &y) },
	} {
		if _, err := op(); err != nil {
			return results{}, err
		}
	}
	return results{add.String(), mul.String(), quo.String(), x.String()}, nil
}
