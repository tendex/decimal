package decimal_test

import (
	"fmt"
	"slices"

	"github.com/tendex/decimal"
)

func Example() {
	price := decimal.MustParse64("19.99")
	qty := decimal.New64(3, 0)
	rate := decimal.MustParse64("0.0825")

	subtotal := price.Mul(qty)
	tax := subtotal.Mul(rate).Round(2)
	fmt.Println("subtotal:", subtotal)
	fmt.Println("tax:     ", tax)
	fmt.Println("total:   ", subtotal.Add(tax))

	// The same sum in binary floating point is not 0.3.
	f, g := 0.1, 0.2
	fmt.Println(decimal.MustParse64("0.1").Add(decimal.MustParse64("0.2")), f+g == 0.3)
	// Output:
	// subtotal: 59.97
	// tax:      4.95
	// total:    64.92
	// 0.3 false
}

func ExampleContext() {
	x, y := decimal.New64(2, 0), decimal.New64(3, 0)

	c := decimal.Context{Rounding: decimal.ToZero}
	fmt.Println(c.Quo64(x, y), c.Flags)

	c = decimal.Context{Rounding: decimal.ToPositiveInf}
	fmt.Println(c.Quo64(x, y), c.Flags)

	c = decimal.Context{}
	fmt.Println(c.Quo64(x, decimal.Decimal64{}), c.Flags)
	fmt.Println(c.Sqrt64(decimal.New64(-1, 0)), c.Flags)
	// Output:
	// 0.6666666666666666 Inexact
	// 0.6666666666666667 Inexact
	// Infinity DivisionByZero
	// NaN Invalid|DivisionByZero
}

func ExampleDecimal64_Equal() {
	a := decimal.MustParse64("1.5")
	b := decimal.MustParse64("1.500")

	fmt.Println(a.Equal(b))       // equal in value
	fmt.Println(a.SameQuantum(b)) // but not in representation
	fmt.Println(a.CmpTotal(b))    // which the total order distinguishes
	fmt.Println(a.Reduce().Bits() == b.Reduce().Bits())
	// Output:
	// true
	// false
	// 1
	// true
}

func ExampleDecimal64_Quantize() {
	x := decimal.MustParse64("2.17")
	fmt.Println(x.Quantize(decimal.MustParse64("0.001")))
	fmt.Println(x.Quantize(decimal.MustParse64("0.1")))
	fmt.Println(x.Quantize(decimal.MustParse64("1E+1")))
	// Output:
	// 2.170
	// 2.2
	// 0E+1
}

func ExampleDecimal64_Format() {
	x := decimal.MustParse64("1234.5")
	fmt.Printf("%v|%.2f|%10.1f|%e|%g\n", x, x, x, x, x)
	// Output:
	// 1234.5|1234.50|    1234.5|1.234500e+03|1234.5
}

func ExampleDecimal64_Cmp() {
	s := []decimal.Decimal64{
		decimal.MustParse64("2.5"),
		decimal.MustParse64("-1"),
		decimal.MustParse64("1E+3"),
		decimal.MustParse64("0.07"),
	}
	slices.SortFunc(s, decimal.Decimal64.Cmp)
	fmt.Println(s)
	// Output:
	// [-1 0.07 2.5 1E+3]
}

func ExampleDecimal64_Parts() {
	neg, coef, exp := decimal.MustParse64("-12.50").Parts()
	fmt.Println(neg, coef, exp)
	// Output:
	// true 1250 -2
}

func ExampleNew128FromFloat() {
	// A float64 cannot hold 0.1; decimal128 has room to show what it holds
	// instead.
	fmt.Println(decimal.New128FromFloat(0.1))
	fmt.Println(decimal.New128FromFloat(0.5))
	// Output:
	// 0.1000000000000000055511151231257827
	// 0.5
}

func ExampleDecimal64_DPD() {
	x := decimal.MustParse64("-7.50")
	fmt.Printf("BID %#016x\nDPD %#016x\n", x.Bits(), x.DPD())
	// Output:
	// BID 0xb1800000000002ee
	// DPD 0xa2300000000003d0
}
