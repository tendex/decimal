package benchmarks

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/tendex/decimal"
)

// Each benchmark runs an operation over n operands, so that branch
// prediction sees a realistic mix rather than a single path, and stores the
// result in a package-level sink so that the call is not optimized away.
const n = 1024

// Sinks shared by the libraries' benchmarks.
var (
	sinkString string
	sinkErr    error
	sinkBool   bool
)

// A workload is a set of operands in plain decimal notation, such as
// "-1234.5678", for libraries of a given precision.
type workload struct {
	name   string
	digits int // significant digits of the libraries compared
	in     [n]string
}

// The workloads use the same operands as the Short and Full benchmarks of
// the decimal package itself, written without exponents:
//
//   - Amounts are everyday amounts: up to nine significant digits, two to
//     four of them after the decimal point.
//   - Digits16 and Digits34 use all 16 or 34 digits, with one to eight
//     before the decimal point, so that most results must be rounded.
var (
	amounts  = newWorkload("Amounts", 16, false)
	digits16 = newWorkload("Digits16", 16, true)
	digits34 = newWorkload("Digits34", 34, true)
)

// newWorkload makes the operands. It draws from the random source exactly as
// benchOperands in the decimal package does.
func newWorkload(name string, prec int, full bool) *workload {
	w := &workload{name: name, digits: prec}
	r := rand.New(rand.NewPCG(42, uint64(prec)))
	for i := range w.in {
		nd := 1 + r.IntN(min(9, prec-1))
		frac := 2 + r.IntN(3)
		if full {
			nd = prec
			frac = nd - (1 + r.IntN(min(8, prec-3)))
		}
		var sign string
		if r.IntN(4) == 0 {
			sign = "-"
		}
		d := make([]byte, nd)
		d[0] = byte('1' + r.IntN(9))
		for j := 1; j < nd; j++ {
			d[j] = byte('0' + r.IntN(10))
		}
		if frac < nd {
			w.in[i] = sign + string(d[:nd-frac]) + "." + string(d[nd-frac:])
		} else {
			w.in[i] = sign + "0." + strings.Repeat("0", frac-nd) + string(d)
		}
	}
	return w
}

// A library adapts one implementation to the benchmarks.
type library struct {
	name string
	// bench registers the sub-benchmarks Add/name, Mul/name, Quo/name,
	// Parse/name and String/name for the workload.
	bench func(b *testing.B, w *workload)
	// results returns what the benchmarked operations compute for operands
	// i and i+1, as strings, for TestAgreement.
	results func(w *workload, i int) (results, error)
}

type results struct {
	add, mul, quo, parsed string
}

// The libraries compared at 16 and at 34 significant digits. The fixed-point
// libraries (govalues, udecimal) hold at most 19 digits, and ANZ implements
// only decimal64, so they appear only in the first group.
var (
	libraries16 = []library{
		{"tendex", benchTendex64, resultsTendex64},
		{"float64", benchFloat64, resultsFloat64},
		{"anz", benchANZ, resultsANZ},
		{"govalues", benchGovalues, resultsGovalues},
		{"udecimal", benchUdecimal, resultsUdecimal},
		{"shopspring", benchShopspring, resultsShopspring},
		{"apd", benchAPD, resultsAPD},
		{"ericlagergren", benchEricLagergren, resultsEricLagergren},
	}
	libraries34 = []library{
		{"tendex", benchTendex128, resultsTendex128},
		{"woodsbury", benchWoodsbury, resultsWoodsbury},
		{"shopspring", benchShopspring, resultsShopspring},
		{"apd", benchAPD, resultsAPD},
		{"ericlagergren", benchEricLagergren, resultsEricLagergren},
	}
)

func BenchmarkAmounts(b *testing.B)  { runLibraries(b, amounts, libraries16) }
func BenchmarkDigits16(b *testing.B) { runLibraries(b, digits16, libraries16) }
func BenchmarkDigits34(b *testing.B) { runLibraries(b, digits34, libraries34) }

func runLibraries(b *testing.B, w *workload, libs []library) {
	for _, lib := range libs {
		lib.bench(b, w)
	}
}

// TestAgreement checks that every library computes, without error, what its
// benchmarks claim to measure: each result must match the exact result, or
// the exact result rounded to the precision of the workload, to within one
// unit in the last place. The libraries differ in how they round, and some
// round quotients to a number of decimal places rather than of significant
// digits, so a tolerance is needed; its purpose is to catch error paths and
// misconfigured contexts, not to test conformance.
func TestAgreement(t *testing.T) {
	for _, c := range []struct {
		w    *workload
		libs []library
	}{
		{amounts, libraries16},
		{digits16, libraries16},
		{digits34, libraries34},
	} {
		// One unit in the last significant place, relative to the value,
		// and one unit in the last decimal place of a quotient rounded to
		// that many places.
		for _, lib := range c.libs {
			rel := decimal.New128(1, 1-c.w.digits)
			abs := decimal.New128(1, -c.w.digits)
			if lib.name == "float64" {
				// Operands and results are rounded in binary, and
				// subtraction can cancel the leading digits, so the
				// error relative to a difference can grow well beyond
				// one unit in the 16th place.
				rel = decimal.New128(1, -9)
			}
			t.Run(c.w.name+"/"+lib.name, func(t *testing.T) {
				for i := range c.w.in {
					x := decimal.MustParse128(c.w.in[i])
					y := decimal.MustParse128(c.w.in[(i+1)%n])
					got, err := lib.results(c.w, i)
					if err != nil {
						t.Fatalf("operands %s, %s: %v", c.w.in[i], c.w.in[(i+1)%n], err)
					}
					for _, op := range []struct {
						name, got string
						want      decimal.Decimal128
					}{
						{"Add", got.add, x.Add(y)},
						{"Mul", got.mul, x.Mul(y)},
						{"Quo", got.quo, x.Quo(y)},
						{"Parse", got.parsed, x},
					} {
						g, err := decimal.Parse128(op.got)
						if err != nil {
							t.Fatalf("%s %s, %s: result %q: %v", op.name, x, y, op.got, err)
						}
						diff := g.Sub(op.want).Abs()
						if diff.Cmp(op.want.Abs().Mul(rel)) > 0 && diff.Cmp(abs) > 0 {
							t.Fatalf("%s %s, %s = %s, want %s", op.name, x, y, op.got, op.want)
						}
					}
				}
			})
		}
	}
}
