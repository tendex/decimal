package decimal

import (
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"strconv"
	"testing"
)

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		coef int64
		exp  int
		want string
	}{
		{0, 0, "0"},
		{1999, -2, "19.99"},
		{-5, 3, "-5E+3"},
		{math.MaxInt64, 0, "9.223372036854776E+18"},
		{math.MinInt64, 0, "-9.223372036854776E+18"},
		{1, 384, "1.000000000000000E+384"},
		{1, 385, "Infinity"},
		{1, math.MaxInt, "Infinity"},
		{1, math.MinInt, "0E-398"},
		{0, math.MinInt, "0E-398"},
	} {
		if got := New64(tt.coef, tt.exp).String(); got != tt.want {
			t.Errorf("New64(%d, %d) = %s, want %s", tt.coef, tt.exp, got, tt.want)
		}
	}
	if got := New128(math.MinInt64, 0).String(); got != "-9223372036854775808" {
		t.Errorf("New128(MinInt64) = %s", got)
	}
	if got := New128FromUint(math.MaxUint64, -2).String(); got != "184467440737095516.15" {
		t.Errorf("New128FromUint = %s", got)
	}
	if got := New32(12345678, 0).String(); got != "1.234568E+7" {
		t.Errorf("New32 = %s", got)
	}
	c := Context{Rounding: ToZero}
	if got := c.New64FromUint(math.MaxUint64, 0).String(); got != "1.844674407370955E+19" || c.Flags != Inexact {
		t.Errorf("Context.New64FromUint = %s [%v]", got, c.Flags)
	}
}

func TestInt64(t *testing.T) {
	for _, tt := range []struct {
		in    string
		want  int64
		exact bool
		uwant uint64
		uex   bool
	}{
		{"0", 0, true, 0, true},
		{"-0", 0, true, 0, true},
		{"1.00", 1, true, 1, true},
		{"1.99", 1, false, 1, false},
		{"-1.99", -1, false, 0, false},
		{"-0.5", 0, false, 0, false},
		{"12E+3", 12000, true, 12000, true},
		{"1E-30", 0, false, 0, false},
		{"9223372036854775807", math.MaxInt64, true, math.MaxInt64, true},
		{"9223372036854775808", math.MaxInt64, false, 1 << 63, true},
		{"-9223372036854775808", math.MinInt64, true, 0, false},
		{"-9223372036854775809", math.MinInt64, false, 0, false},
		{"18446744073709551615", math.MaxInt64, false, math.MaxUint64, true},
		{"18446744073709551615.5", math.MaxInt64, false, math.MaxUint64, false},
		{"18446744073709551616", math.MaxInt64, false, math.MaxUint64, false},
		{"1E+30", math.MaxInt64, false, math.MaxUint64, false},
		{"-1E+30", math.MinInt64, false, 0, false},
		{"Inf", math.MaxInt64, false, math.MaxUint64, false},
		{"-Inf", math.MinInt64, false, 0, false},
		{"NaN", 0, false, 0, false},
	} {
		x := MustParse128(tt.in)
		if got, exact := x.Int64(); got != tt.want || exact != tt.exact {
			t.Errorf("(%s).Int64() = %d, %v; want %d, %v", tt.in, got, exact, tt.want, tt.exact)
		}
		if got, exact := x.Uint64(); got != tt.uwant || exact != tt.uex {
			t.Errorf("(%s).Uint64() = %d, %v; want %d, %v", tt.in, got, exact, tt.uwant, tt.uex)
		}
	}
	for _, tt := range []struct {
		in    string
		mode  RoundingMode
		want  int64
		flags Flags
	}{
		{"2.5", ToNearestEven, 2, Inexact},
		{"2.5", ToNearestAway, 3, Inexact},
		{"-2.5", ToNearestAway, -3, Inexact},
		{"2.1", ToPositiveInf, 3, Inexact},
		{"-2.1", ToPositiveInf, -2, Inexact},
		{"2.9", ToNegativeInf, 2, Inexact},
		{"2.9", ToZero, 2, Inexact},
		{"3.000", ToZero, 3, 0},
		{"9223372036854775807.5", ToNearestEven, math.MaxInt64, Invalid},
		{"9223372036854775807.4", ToNearestEven, math.MaxInt64, Inexact},
		{"-9223372036854775808.5", ToNearestEven, math.MinInt64, Inexact},
		{"sNaN", ToNearestEven, 0, Invalid},
	} {
		c := Context{Rounding: tt.mode}
		if got := c.Int64From128(MustParse128(tt.in)); got != tt.want || c.Flags != tt.flags {
			t.Errorf("Int64From128(%s, %v) = %d [%v]; want %d [%v]", tt.in, tt.mode, got, c.Flags, tt.want, tt.flags)
		}
	}
	r := rand.New(rand.NewPCG(7, 8))
	for range 100000 {
		v := int64(r.Uint64()) >> r.IntN(64)
		if got, exact := New128(v, 0).Int64(); got != v || !exact {
			t.Fatalf("New128(%d).Int64() = %d, %v", v, got, exact)
		}
		if v > -1e16 && v < 1e16 {
			if got, exact := New64(v, 0).Int64(); got != v || !exact {
				t.Fatalf("New64(%d).Int64() = %d, %v", v, got, exact)
			}
		}
	}
}

// bigDecimalText formats the exact value of f to n significant digits,
// rounded to nearest even, in the form d.ddde±dd, using math/big.
func bigDecimalText(f float64, n int) string {
	return new(big.Float).SetFloat64(f).Text('e', n-1)
}

func TestFromFloat(t *testing.T) {
	for _, tt := range []struct {
		f    float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "-0"},
		{1, "1"},
		{-2.5, "-2.5"},
		{1 << 53, "9007199254740992"},
		{0.1, "0.1000000000000000055511151231257827"},
		{0.5, "0.5"},
		{1e23, "99999999999999991611392"},
		{math.MaxFloat64, "1.797693134862315708145274237317044E+308"},
		{math.SmallestNonzeroFloat64, "4.940656458412465441765687928682214E-324"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	} {
		if got := New128FromFloat(tt.f).String(); got != tt.want {
			t.Errorf("New128FromFloat(%g) = %s, want %s", tt.f, got, tt.want)
		}
	}
	if got := New64FromFloat(0.1).String(); got != "0.1000000000000000" {
		t.Errorf("New64FromFloat(0.1) = %s", got)
	}
	if got := New32FromFloat(0.1).String(); got != "0.1000000" {
		t.Errorf("New32FromFloat(0.1) = %s", got)
	}
	var c Context
	if c.New64FromFloat(0.5); c.Flags != 0 {
		t.Errorf("0.5 is exact, got %v", c.Flags)
	}
	if c.New64FromFloat(0.1); c.Flags != Inexact {
		t.Errorf("0.1 is inexact, got %v", c.Flags)
	}
	c = Context{}
	if got := c.New32FromFloat(1e300); !got.IsInf(1) || c.Flags != Overflow|Inexact {
		t.Errorf("New32FromFloat(1e300) = %s [%v]", got, c.Flags)
	}

	r := rand.New(rand.NewPCG(9, 10))
	for i := range 100000 {
		f := math.Float64frombits(r.Uint64())
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		if i%4 == 0 {
			f = float64(r.Int64N(1e9)) / 1e4
		}
		want, _ := strconv.ParseFloat(bigDecimalText(f, 34), 64)
		x := New128FromFloat(f)
		if got := x.Text('e', -1); !sameDecimalText(got, bigDecimalText(f, 34)) {
			t.Fatalf("New128FromFloat(%g) = %s, want %s", f, got, bigDecimalText(f, 34))
		}
		// 34 digits identify any float64, so the conversion round-trips.
		if got := x.Float64(); got != f || want != f {
			t.Fatalf("New128FromFloat(%g).Float64() = %g", f, got)
		}
		if got := New64FromFloat(f).Text('e', -1); !sameDecimalText(got, bigDecimalText(f, 16)) {
			t.Fatalf("New64FromFloat(%g) = %s, want %s", f, got, bigDecimalText(f, 16))
		}
	}
}

// sameDecimalText compares two d.ddde±dd strings as numbers, since the
// decimal types keep trailing zeros that big.Float also prints but short
// exact values do not have.
func sameDecimalText(got, want string) bool {
	a, err1 := Parse128(got)
	b, err2 := Parse128(want)
	return err1 == nil && err2 == nil && a.Equal(b)
}

func TestToFloat(t *testing.T) {
	for _, tt := range []struct {
		in    string
		want  float64
		flags Flags
	}{
		{"0", 0, 0},
		{"1.5", 1.5, 0},
		{"0.1", 0.1, Inexact},
		{"123456789E-3", 123456.789, Inexact},
		{"1E+22", 1e22, 0},
		{"1E+23", 1e23, Inexact},
		{"9007199254740993", 9007199254740992, Inexact},
		{"1E+400", math.Inf(1), Overflow | Inexact},
		{"-1E+400", math.Inf(-1), Overflow | Inexact},
		{"1E-400", 0, Underflow | Inexact},
		{"4.9E-324", 5e-324, Underflow | Inexact},
		// Just below the smallest normal float64 and rounding up to it:
		// tiny before rounding, and the first also after it.
		{"2.225073858507201136057409796709132E-308", 0x1p-1022, Underflow | Inexact},
		{"2.225073858507201383090232717332404E-308", 0x1p-1022, Underflow | Inexact},
		{"2.225073858507201383090232717332405E-308", 0x1p-1022, Inexact},
		{"-2.225073858507201383090232717332404E-308", -0x1p-1022, Underflow | Inexact},
		{"2.225073858507201E-308", 0x1p-1022 - 0x1p-1074, Underflow | Inexact},
		{"Inf", math.Inf(1), 0},
	} {
		x := MustParse128(tt.in)
		var c Context
		if got := c.Float64From128(x); got != tt.want || c.Flags != tt.flags {
			t.Errorf("Float64From128(%s) = %g [%v], want %g [%v]", tt.in, got, c.Flags, tt.want, tt.flags)
		}
		if got := x.Float64(); got != tt.want {
			t.Errorf("(%s).Float64() = %g, want %g", tt.in, got, tt.want)
		}
	}
	if f := MustParse64("-0").Float64(); f != 0 || !math.Signbit(f) {
		t.Errorf("-0 converts to %g", f)
	}
	if f := NaN32().Float64(); !math.IsNaN(f) {
		t.Errorf("NaN converts to %g", f)
	}
	var c Context
	if c.Float64From64(MustParse64("sNaN")); c.Flags != Invalid {
		t.Errorf("sNaN raised %v", c.Flags)
	}
	sNaN := math.Float64frombits(0x7FF0000000000001) // quiet bit clear
	for _, tt := range []struct {
		f    float64
		want string
		flag Flags
	}{
		{math.NaN(), "NaN", 0},
		{math.Copysign(math.NaN(), -1), "-NaN", 0},
		{sNaN, "NaN", Invalid},
		{math.Copysign(sNaN, -1), "-NaN", Invalid},
	} {
		var c32, c64, c128 Context
		for _, got := range []fmt.Stringer{c32.New32FromFloat(tt.f), c64.New64FromFloat(tt.f), c128.New128FromFloat(tt.f)} {
			if got.String() != tt.want {
				t.Errorf("NaN %#x converts to %s, want %s", math.Float64bits(tt.f), got, tt.want)
			}
		}
		if c32.Flags != tt.flag || c64.Flags != tt.flag || c128.Flags != tt.flag {
			t.Errorf("NaN %#x raised %v, %v, %v; want %v", math.Float64bits(tt.f), c32.Flags, c64.Flags, c128.Flags, tt.flag)
		}
	}
	r := rand.New(rand.NewPCG(11, 12))
	for range 100000 {
		x := rand64.fromRef(ref64.random(r))
		if !x.IsFinite() {
			continue
		}
		want, _ := strconv.ParseFloat(x.String(), 64)
		if got := x.Float64(); got != want && !(got == 0 && want == 0) {
			t.Fatalf("(%s).Float64() = %g, want %g", x, got, want)
		}
	}
}

func TestConvertFormat(t *testing.T) {
	for _, tt := range []struct {
		in    string
		want  string
		flags Flags
	}{
		{"1.50", "1.50", 0},
		{"1.2345678", "1.234568", Inexact},
		{"1.2345675", "1.234568", Inexact},
		{"1.2345665", "1.234566", Inexact},
		{"9.9999999E+96", "Infinity", Overflow | Inexact},
		{"1E-101", "1E-101", 0},
		{"1E-102", "0E-101", Underflow | Inexact},
		{"1E+200", "Infinity", Overflow | Inexact},
		{"0E+200", "0E+90", 0},
		{"1E+96", "1.000000E+96", 0},
		{"-Inf", "-Infinity", 0},
		{"NaN123", "NaN123", 0},
		{"NaN1234567", "NaN", 0}, // payload does not fit
		{"-sNaN5", "-NaN5", Invalid},
	} {
		var c Context
		if got := c.Decimal32From64(MustParse64(tt.in)).String(); got != tt.want || c.Flags != tt.flags {
			t.Errorf("Decimal32From64(%s) = %s [%v], want %s [%v]", tt.in, got, c.Flags, tt.want, tt.flags)
		}
		c = Context{}
		if got := c.Decimal32From128(MustParse128(tt.in)).String(); got != tt.want || c.Flags != tt.flags {
			t.Errorf("Decimal32From128(%s) = %s [%v], want %s [%v]", tt.in, got, c.Flags, tt.want, tt.flags)
		}
	}
	c := Context{Rounding: ToPositiveInf}
	if got := c.Decimal64From128(MustParse128("1.00000000000000000000000000000001")).String(); got != "1.000000000000001" {
		t.Errorf("Decimal64From128 rounding up: %s", got)
	}
	if got := MustParse128("NaN1234567890123456").Decimal64().String(); got != "NaN" {
		t.Errorf("NaN payload should be dropped: %s", got)
	}
	// Widening is exact and preserves the representation.
	r := rand.New(rand.NewPCG(13, 14))
	for range 100000 {
		x := New32FromBits(r.Uint32()).Canonical()
		if x.IsSignaling() {
			continue
		}
		if got := x.Decimal64(); got.String() != x.String() || got.Decimal32().Bits() != x.Bits() {
			t.Fatalf("%s widens to %s", x, got)
		}
		if got := x.Decimal128(); got.String() != x.String() || got.Decimal32().Bits() != x.Bits() {
			t.Fatalf("%s widens to %s", x, got)
		}
		y := New64FromBits(r.Uint64()).Canonical()
		if y.IsSignaling() {
			continue
		}
		if got := y.Decimal128(); got.String() != y.String() || got.Decimal64().Bits() != y.Bits() {
			t.Fatalf("%s widens to %s", y, got)
		}
	}
	c = Context{}
	if got := c.Decimal128From32(MustParse32("sNaN9")); got.String() != "NaN9" || c.Flags != Invalid {
		t.Errorf("widening sNaN: %s [%v]", got, c.Flags)
	}
}
