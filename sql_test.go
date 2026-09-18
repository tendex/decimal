package decimal

import (
	"database/sql"
	"database/sql/driver"
	"math"
	"testing"
)

var (
	_ sql.Scanner   = (*Decimal32)(nil)
	_ sql.Scanner   = (*Decimal64)(nil)
	_ sql.Scanner   = (*Decimal128)(nil)
	_ driver.Valuer = Decimal32{}
	_ driver.Valuer = Decimal64{}
	_ driver.Valuer = Decimal128{}
)

func TestValue(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"19.990", "19.990"},
		{"-0", "-0"},
		{"0.00", "0.00"},
		{"1.25E+30", "1250000000000000000000000000000"},
		{"1E-7", "0.0000001"},
		{"-123456789012345.6", "-123456789012345.6"},
		{"Infinity", "Infinity"},
		{"-Infinity", "-Infinity"},
		{"NaN", "NaN"},
	} {
		x := MustParse64(c.in)
		v, err := x.Value()
		if err != nil || v != c.want {
			t.Errorf("Value(%s) = %#v, %v, want %q", c.in, v, err, c.want)
		}
		// database/sql must accept the type as an argument as it is.
		if _, err := driver.DefaultParameterConverter.ConvertValue(x); err != nil {
			t.Errorf("ConvertValue(%s): %v", c.in, err)
		}
	}
	if v, _ := MustParse32("1.5").Value(); v != "1.5" {
		t.Errorf("Decimal32 Value = %#v", v)
	}
	if v, _ := MustParse128("1E+40").Value(); v != "10000000000000000000000000000000000000000" {
		t.Errorf("Decimal128 Value = %#v", v)
	}
}

func TestScan(t *testing.T) {
	for _, c := range []struct {
		src  any
		want string
	}{
		{"1.50", "1.50"},
		{[]byte("-12.345"), "-12.345"},
		{"1E+3", "1E+3"},
		{"Infinity", "Infinity"},
		{int64(-42), "-42"},
		{int64(math.MinInt64), "-9.223372036854776E+18"},
		{uint64(1 << 63), "9.223372036854776E+18"},
		{0.1, "0.1"},
		{float32(0.1), "0.1"},
		{1e300, "1E+300"},
		{5e-324, "5E-324"},
		{math.Inf(-1), "-Infinity"},
		{math.Inf(1), "Infinity"},
		{math.NaN(), "NaN"},
	} {
		var x Decimal64
		if err := x.Scan(c.src); err != nil {
			t.Errorf("Scan(%#v): %v", c.src, err)
		} else if got := x.String(); got != c.want {
			t.Errorf("Scan(%#v) = %s, want %s", c.src, got, c.want)
		}
	}
	for _, src := range []any{nil, true, "", "x", "1.5.0", 42, int32(1)} {
		x := MustParse64("7")
		if err := x.Scan(src); err == nil {
			t.Errorf("Scan(%#v) accepted", src)
		} else if !x.Equal(MustParse64("7")) {
			t.Errorf("Scan(%#v) modified the destination on error: %v", src, x)
		}
	}

	var s Decimal32
	var q Decimal128
	if s.Scan("1.5") != nil || q.Scan([]byte("1.5")) != nil || s.String() != "1.5" || q.String() != "1.5" {
		t.Errorf("Decimal32/Decimal128 Scan: %v %v", s, q)
	}
	if s.Scan(nil) == nil || q.Scan(nil) == nil {
		t.Error("Decimal32/Decimal128 Scan accepted NULL")
	}

	// sql.Null needs no type of its own.
	var n sql.Null[Decimal64]
	if err := n.Scan(nil); err != nil || n.Valid {
		t.Errorf("Null.Scan(nil): %v, valid %v", err, n.Valid)
	}
	if v, err := n.Value(); err != nil || v != nil {
		t.Errorf("Null.Value() of NULL = %#v, %v", v, err)
	}
	if err := n.Scan("2.50"); err != nil || !n.Valid || n.V.String() != "2.50" {
		t.Errorf("Null.Scan(2.50): %v, %+v", err, n)
	}
	if v, err := n.Value(); err != nil || v != "2.50" {
		t.Errorf("Null.Value() = %#v, %v", v, err)
	}
}
