package decimal

import (
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
)

// checkFloatFast checks the fast conversion of coef × 10**exp against the
// conversion through digits, which strconv rounds correctly, and reports
// whether the fast one gave an answer.
func checkFloatFast(t *testing.T, coef uint128, exp int) bool {
	t.Helper()
	got, ok := toFloatFast(coef, exp)
	if !ok {
		return false
	}
	if want := toFloatSlow(coef, exp); got != want {
		t.Fatalf("%v × 10**%d: fast conversion gives %v (%#016x), want %v (%#016x)",
			big128(coef), exp, got, math.Float64bits(got), want, math.Float64bits(want))
	}
	return true
}

// TestFloatFastRandom checks the fast conversion over the whole range of both
// large formats, and reports how often it has to give up.
func TestFloatFastRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(20, 26))
	n, inRange, fast := 0, 0, 0
	for _, f := range []refFormat{ref64, ref128} {
		for i := 0; i < randomN(); i++ {
			v := f.random(r)
			if v.kind != finite || v.coef.Sign() == 0 {
				continue
			}
			n++
			ok := checkFloatFast(t, fromBig(v.coef).low128(), v.exp)
			// Most random values are far outside the range of a float64,
			// where the fast conversion always gives up; the interesting
			// rate is the one for values it can represent.
			if e := v.exp + len(v.coef.String()); -300 < e && e < 300 {
				inRange++
				if ok {
					fast++
				}
			}
		}
	}
	t.Logf("%d values, of which %d in range for a float64; %.2f%% of those converted without falling back",
		n, inRange, 100*float64(fast)/float64(inRange))
}

// TestFloatFastHalfway checks the conversion on the values it finds hardest:
// those just below, at, and just above the midpoint between two consecutive
// float64s, where the rounding is decided by the last digits.
func TestFloatFastHalfway(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 26))
	ten := big.NewInt(10)
	var checked, gaveUp int
	for i := 0; i < 20000; i++ {
		// A random float64 and its neighbour, whose midpoint is a tie.
		f := math.Float64frombits(r.Uint64() &^ (1 << 63))
		if math.IsInf(f, 0) || math.IsNaN(f) || f == 0 {
			continue
		}
		next := math.Nextafter(f, math.Inf(1))
		if math.IsInf(next, 0) {
			continue
		}
		mid := new(big.Rat).Mul(
			new(big.Rat).Add(new(big.Rat).SetFloat64(f), new(big.Rat).SetFloat64(next)),
			big.NewRat(1, 2))

		// The midpoint as a decimal of up to 34 digits: 10**scale × mid,
		// rounded down, is the coefficient of a decimal that is at or just
		// below the tie, and the next one up is just above it.
		exp := 0
		if s := mid.FloatString(0); len(s) > 34 {
			exp = len(s) - 34
		} else {
			// Scale up until the coefficient has 34 digits.
			exp = -(34 - len(s))
			for mid.Denom().Cmp(big.NewInt(1)) != 0 && -exp < 400 {
				exp--
			}
		}
		scale := new(big.Rat).SetInt(new(big.Int).Exp(ten, big.NewInt(int64(max(exp, -exp))), nil))
		scaled := new(big.Rat).Set(mid)
		if exp >= 0 {
			scaled.Quo(scaled, scale)
		} else {
			scaled.Mul(scaled, scale)
		}
		coef := new(big.Int).Quo(scaled.Num(), scaled.Denom())
		if coef.BitLen() > 113 {
			continue
		}
		for _, delta := range []int64{-1, 0, 1} {
			c := new(big.Int).Add(coef, big.NewInt(delta))
			if c.Sign() <= 0 || c.BitLen() > 113 {
				continue
			}
			checked++
			if !checkFloatFast(t, fromBig(c).low128(), exp) {
				gaveUp++
			}
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d values checked", checked)
	}
	t.Logf("%d values at or beside a midpoint, %d fell back", checked, gaveUp)
}

// TestFloatFastRoundTrip checks that every float64 converts to a 34-digit
// decimal and back to itself, through the fast conversion where it applies.
func TestFloatFastRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(22, 26))
	for i := 0; i < 200000; i++ {
		f := math.Float64frombits(r.Uint64())
		if math.IsInf(f, 0) || math.IsNaN(f) {
			continue
		}
		x := MustParse128(strconv.FormatFloat(f, 'e', 33, 64))
		if got := x.Float64(); got != f {
			t.Fatalf("%v -> %v -> %v", f, x, got)
		}
		// The shortest decimal that identifies a float64 needs up to 17
		// digits; where 16 are enough, it round-trips through Decimal64 too.
		s := strconv.FormatFloat(f, 'e', -1, 64)
		if digits := len(s[:strings.IndexByte(s, 'e')]) - strings.Count(s[:2], "-") - strings.Count(s, "."); digits <= 16 {
			if y := MustParse64(s); y.Float64() != f {
				t.Fatalf("%v -> %v -> %v", f, y, y.Float64())
			}
		}
	}
}

// TestFloatFastBoundaries checks the edges of the fast conversion's range.
func TestFloatFastBoundaries(t *testing.T) {
	for _, s := range []string{
		"1E-400", "1E-324", "4.9406564584124654E-324", "2.2250738585072014E-308",
		"2.2250738585072011E-308", "1E-300", "0.1", "1", "1.7976931348623157E+308",
		"1.7976931348623159E+308", "9.9999999999999999E+307", "1E+309", "1E+400",
		"9999999999999999999999999999999999E+300", "1E-6176", "1E+6111",
	} {
		x := MustParse128(s)
		n := x.unpack()
		got := x.Float64()
		// ParseFloat reports the overflowing cases with ±Inf and ErrRange.
		want, err := strconv.ParseFloat(s, 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			t.Fatalf("%s: %v", s, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", s, got, want)
		}
		// Whatever the fast conversion answers must match.
		if f, ok := toFloatFast(n.coef, int(n.exp)); ok && f != want {
			t.Errorf("%s: fast conversion gives %v, want %v", s, f, want)
		}
	}
}
