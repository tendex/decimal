package decimal

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"unsafe"
)

func TestLayout(t *testing.T) {
	for _, tt := range []struct {
		v    any
		size uintptr
	}{
		{Decimal32{}, 4},
		{Decimal64{}, 8},
		{Decimal128{}, 16},
	} {
		typ := reflect.TypeOf(tt.v)
		if typ.Size() != tt.size {
			t.Errorf("%v: size %d, want %d", typ, typ.Size(), tt.size)
		}
		// == would compare representations, not values. Decimal32 is the
		// exception: nothing that forbids == is less than pointer-aligned.
		if typ.Comparable() != (tt.size == 4) {
			t.Errorf("%v: Comparable() = %v", typ, typ.Comparable())
		}
	}
	if unsafe.Sizeof(num{}) != 16 {
		t.Errorf("num is %d bytes; it should pass in registers", unsafe.Sizeof(num{}))
	}
}

func TestZeroValue(t *testing.T) {
	var z32 Decimal32
	var z64 Decimal64
	var z128 Decimal128
	for _, s := range []string{z32.String(), z64.String(), z128.String()} {
		if s != "0" {
			t.Errorf("zero value is %s, want 0", s)
		}
	}
	if b := z64.Bits(); b != 0x31C0000000000000 {
		t.Errorf("zero Decimal64 encodes as %#x", b)
	}
	// Accumulating into a zero value must not disturb the exponent of the
	// sum, which it would if the zero value were the all-zero-bits 0E-398.
	sum := z64
	for _, s := range []string{"1.50", "2.25", "0.25"} {
		sum = sum.Add(MustParse64(s))
	}
	if got := sum.String(); got != "4.00" {
		t.Errorf("sum = %s, want 4.00", got)
	}
	if got := z128.Add(MustParse128("1.5")).String(); got != "1.5" {
		t.Errorf("0 + 1.5 = %s", got)
	}
}

func TestParseString(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"0", "0"},
		{"-0", "-0"},
		{"+1", "1"},
		{"1.50", "1.50"},
		{".5", "0.5"},
		{"5.", "5"},
		{"-12.50", "-12.50"},
		{"1E+3", "1E+3"},
		{"1e3", "1E+3"},
		{"1.25E+3", "1.25E+3"},
		{"125E-2", "1.25"},
		{"0.000001", "0.000001"},
		{"0.0000001", "1E-7"},
		{"0.00", "0.00"},
		{"0E+2", "0E+2"},
		{"00012", "12"},
		{"1234567890123456", "1234567890123456"},
		{"12345678901234567", "1.234567890123457E+16"}, // rounded up
		{"12345678901234565", "1.234567890123456E+16"}, // tie, to even
		{"12345678901234565000000000000000000000000000000001", "1.234567890123457E+49"},
		{"9.999999999999999E+384", "9.999999999999999E+384"},
		{"1E+385", "Infinity"},
		{"1E-398", "1E-398"},
		{"1E-399", "0E-398"},
		{"1E+384", "1.000000000000000E+384"}, // clamped
		{"0E+999999999999", "0E+369"},
		{"0E-999999999999", "0E-398"},
		{"1E+99999999999999999999", "Infinity"},
		{"inf", "Infinity"},
		{"-Infinity", "-Infinity"},
		{"+INF", "Infinity"},
		{"nan", "NaN"},
		{"-NaN", "-NaN"},
		{"NaN123", "NaN123"},
		{"nan000", "NaN"},
		{"sNaN", "sNaN"},
		{"-snan42", "-sNaN42"},
	} {
		x, err := Parse64(tt.in)
		if err != nil {
			t.Errorf("Parse64(%q): %v", tt.in, err)
			continue
		}
		if got := x.String(); got != tt.want {
			t.Errorf("Parse64(%q) = %s, want %s", tt.in, got, tt.want)
		}
		if !x.IsCanonical() {
			t.Errorf("Parse64(%q) is not canonical", tt.in)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{
		"", "+", "-", ".", "e5", "1e", "1e+", "1.2.3", "1,5", " 1", "1 ", "0x10", "1_000",
		"abc", "infinit", "infinityy", "nanx", "NaN1.5", "sna", "--1", "1e5.5",
		"NaN1234567890123456", // payload does not fit
	} {
		x, err := Parse64(in)
		if err == nil {
			t.Errorf("Parse64(%q) = %v, want an error", in, x)
			continue
		}
		var ne *strconv.NumError
		if !errors.As(err, &ne) || !errors.Is(err, strconv.ErrSyntax) || ne.Num != in || ne.Func != "decimal.Parse64" {
			t.Errorf("Parse64(%q): unexpected error %#v", in, err)
		}
		if !x.IsNaN() {
			t.Errorf("Parse64(%q) returned %v with its error, want NaN", in, x)
		}
	}
	var c Context
	if _, err := c.Parse128("bogus"); err == nil || c.Flags != Invalid {
		t.Errorf("Context.Parse128: err = %v, flags = %v", err, c.Flags)
	}
	defer func() {
		if recover() == nil {
			t.Error("MustParse32 did not panic")
		}
	}()
	MustParse32("bogus")
}

func TestContextParse(t *testing.T) {
	c := Context{Rounding: ToZero}
	x, _ := c.Parse64("1.2345678901234567")
	if got := x.String(); got != "1.234567890123456" || c.Flags != Inexact {
		t.Errorf("got %s [%v]", got, c.Flags)
	}
	c = Context{}
	x, _ = c.Parse64("1E-400")
	if got := x.String(); got != "0E-398" || c.Flags != Inexact|Underflow {
		t.Errorf("got %s [%v]", got, c.Flags)
	}
	c = Context{}
	y, _ := c.Parse32("9.9999995E+96")
	if got := y.String(); got != "Infinity" || c.Flags != Inexact|Overflow {
		t.Errorf("got %s [%v]", got, c.Flags)
	}
}

func TestText(t *testing.T) {
	for _, tt := range []struct {
		in     string
		format byte
		prec   int
		want   string
	}{
		{"1.50", 'f', -1, "1.50"},
		{"1.50", 'f', 0, "2"},
		{"2.50", 'f', 0, "2"}, // ties to even
		{"1.50", 'f', 1, "1.5"},
		{"1.50", 'f', 4, "1.5000"},
		{"15E+2", 'f', -1, "1500"},
		{"15E+2", 'f', 2, "1500.00"},
		{"0.00015", 'f', -1, "0.00015"},
		{"0.00015", 'f', 3, "0.000"},
		{"0.0005", 'f', 3, "0.000"},
		{"0.0015", 'f', 3, "0.002"},
		{"0.00051", 'f', 3, "0.001"},
		{"0.0006", 'f', 1, "0.0"},
		{"9.995", 'f', 2, "10.00"}, // exact tie in decimal, to even
		{"9.985", 'f', 2, "9.98"},
		{"999.9", 'f', 0, "1000"},
		{"0.00", 'f', -1, "0.00"},
		{"-0", 'f', 2, "-0.00"},
		{"1234.5678", 'e', -1, "1.2345678e+03"},
		{"1234.5678", 'e', 3, "1.235e+03"},
		{"1234.5678", 'E', 0, "1E+03"},
		{"1.50", 'e', -1, "1.50e+00"},
		{"0", 'e', -1, "0e+00"},
		{"0.00", 'e', -1, "0e-02"}, // an exact zero keeps its exponent, as String does
		{"-0E+5", 'e', -1, "-0e+05"},
		{"0E-398", 'E', -1, "0E-398"},
		{"0E+5", 'f', -1, "0"},
		{"0.00", 'f', -1, "0.00"},
		{"0E-398", 'e', 3, "0.000e+00"},
		{"0", 'e', 2, "0.00e+00"},
		{"1E-7", 'e', -1, "1e-07"},
		{"9.99E+384", 'e', 1, "1.0e+385"},
		{"1E-398", 'e', -1, "1e-398"},
		{"1234.5678", 'g', -1, "1234.5678"},
		{"1234.5678", 'g', 3, "1.23e+03"},
		{"1234.5678", 'g', 6, "1234.57"},
		{"1.50", 'g', -1, "1.50"},
		{"1.50", 'g', 3, "1.5"}, // a precision drops trailing zeros, as strconv does
		{"1.50", 'g', 0, "2"},
		{"1.00", 'g', 3, "1"},
		{"100", 'g', 3, "100"},
		{"1000", 'g', 3, "1e+03"},
		{"100.5", 'g', 3, "100"},
		{"999.9", 'g', 3, "1e+03"},
		{"0", 'g', 3, "0"},
		{"0.00", 'g', 3, "0"},
		{"0E+5", 'g', 3, "0"},
		{"0.00", 'g', -1, "0.00"},
		{"-0.0000", 'g', -1, "-0.0000"},
		{"0E-5", 'g', -1, "0e-05"}, // a zero switches to %e where 1E-5 does
		{"-0E-398", 'g', -1, "-0e-398"},
		{"0E+5", 'g', -1, "0"},
		{"0E+21", 'g', -1, "0e+21"}, // where 1E+21 is 1e+21
		{"0E-398", 'g', 3, "0"},
		{"1E+25", 'g', -1, "1e+25"},
		{"1E+20", 'g', -1, "100000000000000000000"},
		{"0.00001", 'g', -1, "1e-05"},
		{"0.0001", 'g', -1, "0.0001"},
		{"Inf", 'f', 2, "Infinity"},
		{"-Inf", 'e', 2, "-Infinity"},
		{"NaN7", 'g', 2, "NaN7"},
	} {
		if got := MustParse64(tt.in).Text(tt.format, tt.prec); got != tt.want {
			t.Errorf("(%s).Text(%q, %d) = %s, want %s", tt.in, tt.format, tt.prec, got, tt.want)
		}
	}
	if got := MustParse128("0E-6176").Text('g', -1); got != "0e-6176" {
		t.Errorf("(0E-6176).Text('g', -1) = %.40s..., want 0e-6176", got)
	}
	if got := fmt.Sprintf("%g", MustParse32("-0E-101")); got != "-0e-101" {
		t.Errorf("Sprintf(%%g, -0E-101) = %.40s..., want -0e-101", got)
	}
}

func TestFormat(t *testing.T) {
	x := MustParse64("-1234.5678")
	for _, tt := range []struct {
		format string
		v      any
		want   string
	}{
		{"%v", x, "-1234.5678"},
		{"%s", x, "-1234.5678"},
		{"%f", x, "-1234.567800"},
		{"%.2f", x, "-1234.57"},
		{"%12.2f|", x, "    -1234.57|"},
		{"%-12.2f|", x, "-1234.57    |"},
		{"%012.2f|", x, "-00001234.57|"},
		{"%+.1f", x.Neg(), "+1234.6"},
		{"% .1f", x.Neg(), " 1234.6"},
		{"%e", x, "-1.234568e+03"},
		{"%.3E", x, "-1.235E+03"},
		{"%g", x, "-1234.5678"},
		{"%.3g", x, "-1.23e+03"},
		{"%8v|", Inf64(1), "Infinity|"},
		{"%10v|", Inf64(-1), " -Infinity|"},
		{"%010v|", NaN64(), "       NaN|"},
		{"%d", x, "%!d(decimal.Decimal64=-1234.5678)"},
		{"%v", MustParse32("1.5"), "1.5"},
		{"%.1f", MustParse128("0.25"), "0.2"},
		{"%v", []Decimal64{New64(15, -1), New64(2, 0)}, "[1.5 2]"},
		{"%+v", x.Neg(), "1234.5678"}, // %+v asks for field names, as for float64
		{"%+v", x, "-1234.5678"},
		{"%+v", []Decimal64{New64(15, -1)}, "[1.5]"},
		{"%+v", struct{ D Decimal128 }{New128(15, -1)}, "{D:1.5}"},
		{"%+v", MustParse32("1.5"), "1.5"},
		{"%+s", x.Neg(), "+1234.5678"},
		{"% v", x.Neg(), " 1234.5678"},
	} {
		if got := fmt.Sprintf(tt.format, tt.v); got != tt.want {
			t.Errorf("Sprintf(%q) = %q, want %q", tt.format, got, tt.want)
		}
	}
}

// TestFormatStrconv checks the explicit-precision formats against strconv on
// values that float64 and Decimal64 both represent exactly, so that the two
// round the same digits.
func TestFormatStrconv(t *testing.T) {
	var vals []float64
	for _, v := range []float64{0, 1, 5, 9, 100, 999, 1000, 1024, 12345, 100000, 1234567, 1 << 53, 1e20, 1e22} {
		vals = append(vals, v, -v)
	}
	for f := 1.0; f >= 0x1p-10; f /= 2 { // dyadic fractions with short expansions
		vals = append(vals, f, 1+f, 100+f, 999+f, 0x1p40*f)
	}
	for _, f := range vals {
		x := New64FromFloat(f)
		for _, verb := range "efg" {
			for prec := 0; prec <= 12; prec++ {
				format := fmt.Sprintf("%%.%d%c", prec, verb)
				if got, want := fmt.Sprintf(format, x), fmt.Sprintf(format, f); got != want {
					t.Errorf("Sprintf(%q, %v) = %q, want %q", format, x, got, want)
				}
			}
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	for i := 0; i < 200000; i++ {
		x := New64FromBits(r.Uint64()).Canonical()
		if i%3 == 0 {
			x = rand64.fromRef(ref64.random(r))
		}
		y, err := Parse64(x.String())
		if err != nil || y.Bits() != x.Bits() {
			t.Fatalf("%#x -> %s -> %#x (%v)", x.Bits(), x, y.Bits(), err)
		}
		if d := New64FromDPD(x.DPD()); d.Bits() != x.Bits() {
			t.Fatalf("%s: DPD round trip gives %s", x, d)
		}
		for _, format := range []byte{'e', 'f', 'g'} {
			if !x.IsFinite() || format == 'f' && (x.Exponent() > 40 || x.Exponent() < -40) {
				continue
			}
			z, err := Parse64(x.Text(format, -1))
			if err != nil || !(z.Equal(x) || z.IsZero() && x.IsZero()) {
				t.Fatalf("%s -> %s -> %s (%v)", x, x.Text(format, -1), z, err)
			}
		}

		hi, lo := r.Uint64(), r.Uint64()
		w := New128FromBits(hi, lo).Canonical()
		if i%3 == 0 {
			w = rand128.fromRef(ref128.random(r))
		}
		v, err := Parse128(w.String())
		if whi, wlo := w.Bits(); err != nil || !sameBits128(v, w) {
			t.Fatalf("%#x %#x -> %s -> %s (%v)", whi, wlo, w, v, err)
		}
		if d := New128FromDPD(w.DPD()); !sameBits128(d, w) {
			t.Fatalf("%s: DPD round trip gives %s", w, d)
		}

		s := New32FromBits(r.Uint32()).Canonical()
		u, err := Parse32(s.String())
		if err != nil || u.Bits() != s.Bits() {
			t.Fatalf("%#x -> %s -> %#x (%v)", s.Bits(), s, u.Bits(), err)
		}
		if d := New32FromDPD(s.DPD()); d.Bits() != s.Bits() {
			t.Fatalf("%s: DPD round trip gives %s", s, d)
		}
	}
}

func sameBits128(x, y Decimal128) bool {
	xh, xl := x.Bits()
	yh, yl := y.Bits()
	return xh == yh && xl == yl
}

func TestNonCanonical(t *testing.T) {
	for _, tt := range []struct {
		bits uint64
		want string
	}{
		{0x6C7386F26FC0FFFF, "9999999999999999"}, // largest large-form coefficient
		{0x6C7386F26FC10000, "0"},                // 10**16: non-canonical zero
		{0x6C77FFFFFFFFFFFF, "0"},                // all coefficient bits set
		{0xEC77FFFFFFFFFFFF, "-0"},
		{0x7800000000000123, "Infinity"}, // junk below an infinity
		{0x7C00000000000000 | 999999999999999, "NaN999999999999999"},
		{0x7C00000000000000 | 1000000000000000, "NaN"}, // payload too large
		{0x7DFC000000000000 | 5, "NaN5"},               // junk in the reserved bits
		{0x7E00000000000000 | 1<<50 | 7, "sNaN7"},
	} {
		x := New64FromBits(tt.bits)
		if got := x.String(); got != tt.want {
			t.Errorf("%#x: got %s, want %s", tt.bits, got, tt.want)
		}
		canonical := tt.want == "9999999999999999" || tt.want == "NaN999999999999999"
		if x.IsCanonical() != canonical {
			t.Errorf("%#x: IsCanonical = %v", tt.bits, !canonical)
		}
		if c := x.Canonical(); !c.IsCanonical() || c.String() != tt.want {
			t.Errorf("%#x: Canonical = %s (%#x)", tt.bits, c, c.Bits())
		}
		if x.Bits() != tt.bits {
			t.Errorf("%#x: Bits does not round-trip", tt.bits)
		}
		// Arithmetic sees the value, and produces canonical results.
		if got := x.Add(Decimal64{}); !x.IsNaN() && !got.IsCanonical() {
			t.Errorf("%#x + 0 = %#x is not canonical", tt.bits, got.Bits())
		}
	}
	// Every decimal128 encoding in large-coefficient form is a zero.
	if x := New128FromBits(0x6000000000000000|1234, 5678); !x.IsZero() || x.IsCanonical() {
		t.Errorf("large-form decimal128: %s", x)
	}
	if x := New128FromBits(0x0001ED09BEAD87C0, 0x378D8E6400000000); !x.IsZero() || x.IsCanonical() {
		t.Errorf("decimal128 coefficient 10**34 should be a non-canonical zero: %s", x)
	}
	if x := New128FromBits(0x0001ED09BEAD87C0, 0x378D8E63FFFFFFFF); x.IsZero() || !x.IsCanonical() {
		t.Errorf("decimal128 coefficient 10**34-1: %s", x)
	}
}

func TestPredicates(t *testing.T) {
	for _, tt := range []struct {
		in                            string
		class                         Class
		sign                          int
		finite, zero, normal, integer bool
	}{
		{"0", PositiveZero, 0, true, true, false, true},
		{"-0.00", NegativeZero, 0, true, true, false, true},
		{"1", PositiveNormal, 1, true, false, true, true},
		{"-1.50", NegativeNormal, -1, true, false, true, false},
		{"1.00", PositiveNormal, 1, true, false, true, true},
		{"1E+10", PositiveNormal, 1, true, false, true, true},
		{"1E-383", PositiveNormal, 1, true, false, true, false},
		{"9.99E-384", PositiveSubnormal, 1, true, false, false, false},
		{"-1E-398", NegativeSubnormal, -1, true, false, false, false},
		{"Inf", PositiveInf, 1, false, false, false, false},
		{"-Inf", NegativeInf, -1, false, false, false, false},
		{"NaN", QuietNaN, 0, false, false, false, false},
		{"-sNaN", SignalingNaN, 0, false, false, false, false},
	} {
		x := MustParse64(tt.in)
		if x.Class() != tt.class || x.Sign() != tt.sign || x.IsFinite() != tt.finite ||
			x.IsZero() != tt.zero || x.IsNormal() != tt.normal || x.IsInteger() != tt.integer {
			t.Errorf("%s: class %v sign %d finite %v zero %v normal %v integer %v", tt.in,
				x.Class(), x.Sign(), x.IsFinite(), x.IsZero(), x.IsNormal(), x.IsInteger())
		}
		w := x.Decimal128()
		if tt.finite && (w.Sign() != tt.sign || w.IsZero() != tt.zero || w.IsInteger() != tt.integer) {
			t.Errorf("%s as Decimal128: sign %d zero %v integer %v", tt.in, w.Sign(), w.IsZero(), w.IsInteger())
		}
	}
	x := MustParse64("-sNaN")
	if !x.IsNaN() || !x.IsSignaling() || !x.Signbit() || x.IsInf(0) {
		t.Error("sNaN predicates")
	}
	if !Inf64(-1).IsInf(-1) || Inf64(-1).IsInf(1) || !Inf64(1).IsInf(0) || !Inf128(-1).IsInf(-1) || !Inf32(1).IsInf(1) {
		t.Error("IsInf")
	}
	if got := NewNaN64(42, true).String(); got != "sNaN42" {
		t.Errorf("NewNaN64 = %s", got)
	}
	if neg, coef, exp := MustParse64("-12.50").Parts(); !neg || coef != 1250 || exp != -2 {
		t.Errorf("Parts = %v %d %d", neg, coef, exp)
	}
}

func TestCompareAndSort(t *testing.T) {
	s := []Decimal64{
		MustParse64("3"), NaN64(), MustParse64("-Inf"), MustParse64("1.0"),
		MustParse64("1E+2"), MustParse64("-0"), MustParse64("Inf"), MustParse64("-2.5"),
	}
	slices.SortFunc(s, Decimal64.Cmp)
	if got := fmt.Sprint(s); got != "[NaN -Infinity -2.5 -0 1.0 3 1E+2 Infinity]" {
		t.Errorf("sorted: %s", got)
	}
	one, uno := MustParse64("1"), MustParse64("1.000")
	if !one.Equal(uno) || one.Less(uno) || one.Cmp(uno) != 0 || one.CmpTotal(uno) != 1 || one.SameQuantum(uno) {
		t.Error("cohort members must be equal in value and distinct in total order")
	}
	if NaN64().Equal(NaN64()) || NaN64().Less(one) || NaN64().Compare(one) != Unordered {
		t.Error("NaN comparisons")
	}
	if !MustParse64("-0").Equal(Decimal64{}) {
		t.Error("-0 != +0")
	}
	if got := one.Reduce().Bits(); got != uno.Reduce().Bits() {
		t.Error("Reduce does not unify a cohort")
	}
	var c Context
	if c.Compare64(one, NaN64()); c.Flags != 0 {
		t.Errorf("quiet compare raised %v", c.Flags)
	}
	if c.CompareSignal64(one, NaN64()); c.Flags != Invalid {
		t.Errorf("signaling compare raised %v", c.Flags)
	}
	if got := one.Min(NaN64()); !got.IsNaN() {
		t.Errorf("Min(1, NaN) = %s", got)
	}
	if got := c.MinNum64(one, NaN64()); !got.Equal(one) {
		t.Errorf("MinNum(1, NaN) = %s", got)
	}
	if got := MustParse64("-0").Max(Decimal64{}); got.Signbit() {
		t.Errorf("Max(-0, +0) = %s", got)
	}
}

func TestRound(t *testing.T) {
	for _, tt := range []struct {
		in     string
		places int
		want   string
	}{
		{"1.2345", 2, "1.23"},
		{"1.235", 2, "1.24"},
		{"1.225", 2, "1.22"},
		{"1.5", 2, "1.5"}, // never pads
		{"1234", -2, "1.2E+3"},
		{"1250", -2, "1.2E+3"},
		{"-1.5", 0, "-2"},
		{"0.004", 2, "0.00"},
		{"Inf", 2, "Infinity"},
		{"NaN", 2, "NaN"},
		{"123.45", math.MaxInt, "123.45"},
		{"123.45", -369, "0E+369"},
		{"123.45", -370, "NaN"}, // beyond the exponent range
		{"123.45", math.MinInt, "NaN"},
	} {
		if got := MustParse64(tt.in).Round(tt.places).String(); got != tt.want {
			t.Errorf("(%s).Round(%d) = %s, want %s", tt.in, tt.places, got, tt.want)
		}
	}
	for _, places := range []int{-6112, math.MinInt + 1, math.MinInt} { // beyond every format's emax
		var c Context
		if got := c.Round64(MustParse64("123.45"), places); !got.IsNaN() || c.Flags != Invalid {
			t.Errorf("Round64(123.45, %d) = %s [%v], want NaN [Invalid]", places, got, c.Flags)
		}
		var c32 Context
		if got := c32.Round32(MustParse32("123.45"), places); !got.IsNaN() || c32.Flags != Invalid {
			t.Errorf("Round32(123.45, %d) = %s [%v], want NaN [Invalid]", places, got, c32.Flags)
		}
		var c128 Context
		if got := c128.Round128(MustParse128("123.45"), places); !got.IsNaN() || c128.Flags != Invalid {
			t.Errorf("Round128(123.45, %d) = %s [%v], want NaN [Invalid]", places, got, c128.Flags)
		}
	}
	c := Context{Rounding: ToNearestAway}
	if got := c.Round64(MustParse64("1.225"), 2).String(); got != "1.23" || c.Flags != Inexact {
		t.Errorf("half-up round: %s [%v]", got, c.Flags)
	}
	if got := MustParse64("1.5").Quantize(MustParse64("0.001")).String(); got != "1.500" {
		t.Errorf("Quantize pads: %s", got)
	}
	if got := MustParse64("2.5").RoundToIntegral(ToNearestEven).String(); got != "2" {
		t.Errorf("RoundToIntegral: %s", got)
	}
}

func TestMarshal(t *testing.T) {
	type doc struct {
		Price Decimal64  `json:"price"`
		Qty   Decimal32  `json:"qty"`
		Big   Decimal128 `json:"big"`
	}
	in := doc{MustParse64("19.990"), MustParse32("3"), MustParse128("1E+6000")}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"price":"19.990","qty":"3","big":"1E+6000"}`; string(data) != want {
		t.Errorf("json: %s, want %s", data, want)
	}
	var out doc
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Price.Bits() != in.Price.Bits() || out.Qty.Bits() != in.Qty.Bits() || !sameBits128(out.Big, in.Big) {
		t.Errorf("json round trip: %+v", out)
	}
	if err := json.Unmarshal([]byte(`{"price":"x"}`), &out); err == nil {
		t.Error("json: bad number accepted")
	}

	b64, _ := in.Price.MarshalBinary()
	b32, _ := in.Qty.MarshalBinary()
	b128, _ := in.Big.MarshalBinary()
	if len(b32) != 4 || len(b64) != 8 || len(b128) != 16 {
		t.Fatalf("binary sizes %d %d %d", len(b32), len(b64), len(b128))
	}
	var p Decimal64
	var q Decimal32
	var g Decimal128
	if p.UnmarshalBinary(b64) != nil || q.UnmarshalBinary(b32) != nil || g.UnmarshalBinary(b128) != nil {
		t.Fatal("UnmarshalBinary failed")
	}
	if p.Bits() != in.Price.Bits() || q.Bits() != in.Qty.Bits() || !sameBits128(g, in.Big) {
		t.Error("binary round trip")
	}
	if p.UnmarshalBinary(b32) == nil || q.UnmarshalBinary(b64) == nil || g.UnmarshalBinary(b64) == nil {
		t.Error("UnmarshalBinary accepted the wrong length")
	}
}

func TestRoundToMultiple(t *testing.T) {
	for _, tt := range []struct {
		x, y  string
		mode  RoundingMode
		want  string
		flags Flags
	}{
		{"10.12", "0.05", ToNearestEven, "10.10", Inexact},
		{"10.14", "0.05", ToNearestEven, "10.15", Inexact},
		{"10.15", "0.05", ToNearestEven, "10.15", 0},
		{"10.1", "0.05", ToNearestEven, "10.10", 0}, // exact, with a digit added
		{"10.125", "0.05", ToNearestEven, "10.10", Inexact},
		{"10.175", "0.05", ToNearestEven, "10.20", Inexact},
		{"10.125", "0.05", ToNearestAway, "10.15", Inexact},
		{"10.12", "0.05", ToZero, "10.10", Inexact},
		{"10.12", "0.05", ToPositiveInf, "10.15", Inexact},
		{"10.12", "0.05", ToNegativeInf, "10.10", Inexact},
		{"10.12", "0.05", AwayFromZero, "10.15", Inexact},
		{"-10.12", "0.05", ToNearestEven, "-10.10", Inexact},
		{"-10.12", "0.05", ToZero, "-10.10", Inexact},
		{"-10.12", "0.05", ToPositiveInf, "-10.10", Inexact},
		{"-10.12", "0.05", ToNegativeInf, "-10.15", Inexact},
		{"-10.12", "0.05", AwayFromZero, "-10.15", Inexact},
		{"10.12", "0.050", ToNearestEven, "10.100", Inexact}, // the exponent of y
		{"7", "2.5", ToNearestEven, "7.5", Inexact},
		{"6.25", "2.5", ToNearestEven, "5.0", Inexact},
		{"6.25", "2.5", ToNearestAway, "7.5", Inexact},
		{"1234.5", "25", ToNearestEven, "1225", Inexact},
		{"1234.5", "1E+2", ToNearestEven, "1.2E+3", Inexact},
		{"1234.5", "0.01", ToNearestEven, "1234.50", 0},
		{"1234.567", "0.01", ToNearestEven, "1234.57", Inexact},
		// Zeros and values below half an increment.
		{"0", "0.05", ToNearestEven, "0.00", 0},
		{"-0", "0.05", ToNearestEven, "-0.00", 0},
		{"0E+5", "0.05", ToNearestEven, "0.00", 0},
		{"0.001", "0.05", ToNearestEven, "0.00", Inexact},
		{"-0.001", "0.05", ToNearestEven, "-0.00", Inexact},
		{"0.001", "0.05", ToPositiveInf, "0.05", Inexact},
		{"-0.001", "0.05", AwayFromZero, "-0.05", Inexact},
		{"0.025", "0.05", ToNearestEven, "0.00", Inexact},
		{"0.025", "0.05", ToNearestAway, "0.05", Inexact},
		{"1E-20", "0.05", ToNearestEven, "0.00", Inexact},
		{"1E-20", "0.05", AwayFromZero, "0.05", Inexact},
		{"1E-300", "1E+300", ToNegativeInf, "0E+300", Inexact},
		{"-1E-300", "1E+300", ToNegativeInf, "-1E+300", Inexact},
		// The limits of the format.
		{"1E+16", "3", ToNearestEven, "9999999999999999", Inexact},
		{"1E+16", "7", ToNearestEven, "NaN", Invalid},
		{"1E+16", "7", ToZero, "9999999999999996", Inexact},
		{"9999999999999999", "0.05", ToNearestEven, "NaN", Invalid},
		{"1E+20", "0.05", ToNearestEven, "NaN", Invalid},
		{"9.999999999999999E+384", "1", ToNearestEven, "NaN", Invalid},
		{"9.999999999999999E+384", "1E+369", ToNearestEven, "9.999999999999999E+384", 0},
		// Invalid operands.
		{"1", "0", ToNearestEven, "NaN", Invalid},
		{"1", "-0.05", ToNearestEven, "NaN", Invalid},
		{"1", "-0", ToNearestEven, "NaN", Invalid},
		{"1", "Inf", ToNearestEven, "NaN", Invalid},
		{"Inf", "0.05", ToNearestEven, "NaN", Invalid},
		{"Inf", "Inf", ToNearestEven, "NaN", Invalid},
		{"NaN", "0.05", ToNearestEven, "NaN", 0},
		{"1", "NaN7", ToNearestEven, "NaN7", 0},
		{"sNaN", "0.05", ToNearestEven, "NaN", Invalid},
	} {
		c := Context{Rounding: tt.mode}
		got := c.RoundToMultiple64(MustParse64(tt.x), MustParse64(tt.y))
		if got.String() != tt.want || c.Flags != tt.flags {
			t.Errorf("RoundToMultiple64(%s, %s) %v = %s [%v], want %s [%v]", tt.x, tt.y, tt.mode, got, c.Flags, tt.want, tt.flags)
		}
	}
	// A power of ten is Quantize.
	for _, s := range []string{"1234.567", "-0.005", "0.015", "1E+20", "NaN3", "Inf"} {
		x, y := MustParse64(s), MustParse64("0.01")
		if a, b := x.RoundToMultiple(y), x.Quantize(y); a.CmpTotal(b) != 0 {
			t.Errorf("RoundToMultiple(%s, 0.01) = %v, Quantize = %v", s, a, b)
		}
	}
	if got := MustParse64("10.12").RoundToMultiple(MustParse64("0.05")).String(); got != "10.10" {
		t.Errorf("RoundToMultiple: %s", got)
	}
	if got := MustParse32("10.12").RoundToMultiple(MustParse32("0.05")).String(); got != "10.10" {
		t.Errorf("Decimal32 RoundToMultiple: %s", got)
	}
	c := Context{Rounding: ToNearestEven}
	if got := c.RoundToMultiple128(MustParse128("1E+34"), MustParse128("3")).String(); got != "9999999999999999999999999999999999" || c.Flags != Inexact {
		t.Errorf("Decimal128 RoundToMultiple: %s [%v]", got, c.Flags)
	}
	c = Context{Rounding: AwayFromZero}
	if got := c.RoundToMultiple128(MustParse128("1E-6000"), MustParse128("2.5E+6000")).String(); got != "2.5E+6000" || c.Flags != Inexact {
		t.Errorf("Decimal128 RoundToMultiple: %s [%v]", got, c.Flags)
	}
}

func TestAwayFromZero(t *testing.T) {
	c := Context{Rounding: AwayFromZero}
	for _, tt := range []struct{ got, want string }{
		{c.Round64(MustParse64("2.341"), 2).String(), "2.35"},
		{c.Round64(MustParse64("-2.341"), 2).String(), "-2.35"},
		{c.Round64(MustParse64("0.001"), 2).String(), "0.01"},
		{c.Quo64(New64(1, 0), New64(3, 0)).String(), "0.3333333333333334"},
		{c.Quo64(New64(-1, 0), New64(3, 0)).String(), "-0.3333333333333334"},
		{c.Sqrt64(New64(2, 0)).String(), "1.414213562373096"},
		{c.Mul64(MustParse64("9.999999999999999E+384"), New64(10, 0)).String(), "Infinity"},
		{MustParse64("2.1").RoundToIntegral(AwayFromZero).String(), "3"},
		{MustParse64("-2.1").RoundToIntegral(AwayFromZero).String(), "-3"},
		{AwayFromZero.String(), "AwayFromZero"},
	} {
		if tt.got != tt.want {
			t.Errorf("got %s, want %s", tt.got, tt.want)
		}
	}
	if v, exact := c.Int64From64(MustParse64("2.1")), c.Flags&Inexact != 0; v != 3 || !exact {
		t.Errorf("Int64From64 away from zero: %d, inexact %v", v, exact)
	}
}

// TestUnmarshalTextAllocs pins the byte-slice paths at zero allocations: a
// slice is parsed in place, however long, rather than through a string.
func TestUnmarshalTextAllocs(t *testing.T) {
	// The inputs are made outside the closures: a []byte of a constant longer
	// than 32 bytes is itself an allocation.
	short, amount := []byte("-1234.567"), []byte("-1234567890.123456")
	long := []byte("-1234567890123456789012.345678901234E-6000")
	// A decimal128 payload holds 33 digits at most.
	payload64, payload128 := []byte("sNaN123456789012345"), []byte("-sNaN123456789012345678901234567890123")
	// Boxed once, as a driver delivers it.
	var src any = long
	var s Decimal32
	var d Decimal64
	var q Decimal128
	for _, c := range []struct {
		name string
		f    func()
	}{
		{"UnmarshalText32", func() { s.UnmarshalText(short) }},
		{"UnmarshalText64", func() { d.UnmarshalText(amount) }},
		{"UnmarshalText64 payload", func() { d.UnmarshalText(payload64) }},
		{"UnmarshalText128", func() { q.UnmarshalText(long) }},
		{"UnmarshalText128 payload", func() { q.UnmarshalText(payload128) }},
		{"Scan128", func() { q.Scan(src) }},
	} {
		if n := testing.AllocsPerRun(100, c.f); n != 0 {
			t.Errorf("%s allocates %v times", c.name, n)
		}
	}
	if q.String() != "-1.234567890123456789012345678901234E-5979" || !d.IsNaN() {
		t.Errorf("parsed %v, %v", q, d)
	}
}
