// Command xcheck prints random decimal64 and decimal128 operations with the
// results this package computes, for an independent implementation to
// verify. verify.py checks them against Python's decimal module (libmpdec):
//
//	go run ./internal/xcheck 100000 | python3 internal/xcheck/verify.py
//
// The argument is the number of operand sets; each yields 28 operations, in
// a random rounding mode, flags included.
package main

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"

	d "github.com/tendex/decimal"
)

var modes = []string{"ROUND_HALF_EVEN", "ROUND_HALF_UP", "ROUND_DOWN", "ROUND_CEILING", "ROUND_FLOOR"}

func operand(r *rand.Rand, prec, emax int) string {
	switch r.IntN(30) {
	case 0:
		return []string{"Infinity", "-Infinity"}[r.IntN(2)]
	case 1:
		return []string{"NaN", "-NaN12", "sNaN", "sNaN7"}[r.IntN(4)]
	}
	nd := 1 + r.IntN(prec)
	if r.IntN(3) == 0 {
		nd = prec
	}
	b := []byte{}
	if r.IntN(2) == 0 {
		b = append(b, '-')
	}
	switch r.IntN(8) {
	case 0:
		for i := 0; i < nd; i++ {
			b = append(b, '9')
		}
	case 1:
		b = append(b, '1')
		for i := 1; i < nd; i++ {
			b = append(b, '0')
		}
	case 2:
		b = append(b, '5')
		for i := 1; i < nd; i++ {
			b = append(b, '0')
		}
	case 3:
		b = append(b, '0')
	default:
		for i := 0; i < nd; i++ {
			b = append(b, byte('0'+r.IntN(10)))
		}
	}
	var exp int
	etiny := -(emax - 1) - (prec - 1)
	switch r.IntN(10) {
	case 0:
		exp = etiny + r.IntN(2*prec)
	case 1:
		exp = emax - prec + 1 - r.IntN(2*prec)
	case 2, 3:
		exp = r.IntN(4*prec) - 2*prec
	default:
		exp = r.IntN(9) - 6
	}
	return string(b) + "E" + strconv.Itoa(exp)
}

func main() {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	n, _ := strconv.Atoi(os.Args[1])
	r := rand.New(rand.NewPCG(2026, 917))
	for i := 0; i < n; i++ {
		m := r.IntN(5)
		mode := d.RoundingMode(m)
		{
			x, y, z := operand(r, 16, 384), operand(r, 16, 384), operand(r, 16, 384)
			if r.IntN(4) == 0 {
				y = x
			}
			X, Y, Z := d.MustParse64(x), d.MustParse64(y), d.MustParse64(z)
			emit := func(op string, f func(c *d.Context) d.Decimal64, args ...string) {
				c := d.Context{Rounding: mode}
				res := f(&c)
				fmt.Fprintf(w, "64 %s %s %v -> %s %d\n", op, modes[m], args, res, c.Flags)
			}
			emit("add", func(c *d.Context) d.Decimal64 { return c.Add64(X, Y) }, x, y)
			emit("subtract", func(c *d.Context) d.Decimal64 { return c.Sub64(X, Y) }, x, y)
			emit("multiply", func(c *d.Context) d.Decimal64 { return c.Mul64(X, Y) }, x, y)
			emit("divide", func(c *d.Context) d.Decimal64 { return c.Quo64(X, Y) }, x, y)
			emit("fma", func(c *d.Context) d.Decimal64 { return c.FMA64(X, Y, Z) }, x, y, z)
			emit("sqrt", func(c *d.Context) d.Decimal64 { return c.Sqrt64(X) }, x)
			emit("remainder_near", func(c *d.Context) d.Decimal64 { return c.Remainder64(X, Y) }, x, y)
			emit("remainder", func(c *d.Context) d.Decimal64 { return c.Mod64(X, Y) }, x, y)
			emit("quantize", func(c *d.Context) d.Decimal64 { return c.Quantize64(X, Y) }, x, y)
			emit("to_integral_exact", func(c *d.Context) d.Decimal64 { return c.RoundToIntegralExact64(X) }, x)
			emit("next_plus", func(c *d.Context) d.Decimal64 { return c.NextUp64(X) }, x)
			emit("next_minus", func(c *d.Context) d.Decimal64 { return c.NextDown64(X) }, x)
			emit("logb", func(c *d.Context) d.Decimal64 { return c.LogB64(X) }, x)
			emit("normalize", func(c *d.Context) d.Decimal64 { return c.Reduce64(X) }, x)
		}
		{
			x, y, z := operand(r, 34, 6144), operand(r, 34, 6144), operand(r, 34, 6144)
			if r.IntN(4) == 0 {
				y = x
			}
			X, Y, Z := d.MustParse128(x), d.MustParse128(y), d.MustParse128(z)
			emit := func(op string, f func(c *d.Context) d.Decimal128, args ...string) {
				c := d.Context{Rounding: mode}
				res := f(&c)
				fmt.Fprintf(w, "128 %s %s %v -> %s %d\n", op, modes[m], args, res, c.Flags)
			}
			emit("add", func(c *d.Context) d.Decimal128 { return c.Add128(X, Y) }, x, y)
			emit("subtract", func(c *d.Context) d.Decimal128 { return c.Sub128(X, Y) }, x, y)
			emit("multiply", func(c *d.Context) d.Decimal128 { return c.Mul128(X, Y) }, x, y)
			emit("divide", func(c *d.Context) d.Decimal128 { return c.Quo128(X, Y) }, x, y)
			emit("fma", func(c *d.Context) d.Decimal128 { return c.FMA128(X, Y, Z) }, x, y, z)
			emit("sqrt", func(c *d.Context) d.Decimal128 { return c.Sqrt128(X) }, x)
			emit("remainder_near", func(c *d.Context) d.Decimal128 { return c.Remainder128(X, Y) }, x, y)
			emit("remainder", func(c *d.Context) d.Decimal128 { return c.Mod128(X, Y) }, x, y)
			emit("quantize", func(c *d.Context) d.Decimal128 { return c.Quantize128(X, Y) }, x, y)
			emit("to_integral_exact", func(c *d.Context) d.Decimal128 { return c.RoundToIntegralExact128(X) }, x)
			emit("next_plus", func(c *d.Context) d.Decimal128 { return c.NextUp128(X) }, x)
			emit("next_minus", func(c *d.Context) d.Decimal128 { return c.NextDown128(X) }, x)
			emit("logb", func(c *d.Context) d.Decimal128 { return c.LogB128(X) }, x)
			emit("normalize", func(c *d.Context) d.Decimal128 { return c.Reduce128(X) }, x)
		}
	}
}
