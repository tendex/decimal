package decimal

import (
	"math/rand/v2"
	"strconv"
)

// The benchmarks in bench64_test.go, and in bench32_test.go and
// bench128_test.go which are generated from it, time each operation over a
// fixed set of operands, so that branch prediction sees a realistic mix
// rather than a single path:
//
//   - Short operands are everyday amounts: up to nine significant digits
//     (six for Decimal32), two to four of them after the decimal point.
//   - Full operands use the whole precision of the format, with one to
//     eight digits before the decimal point (one to four for Decimal32), so
//     that most results must be rounded.
//
// Each loop calls the method directly, as user code would, so that the
// compiler can inline it, and stores the result in a package-level sink so
// that the call is not optimized away. testing.B.Loop keeps results alive by
// other means, which cost about half a nanosecond per iteration: a tenth of
// the time of the fastest operations here.
//
// The results in the README are produced by the benchmarks module; see
// benchmarks/README.md.

const benchN = 1024

// The precision of each format, for choosing operands.
const (
	digits32  = 7
	digits64  = 16
	digits128 = 34
)

var (
	sinkInt   int
	sinkInt64 int64
	sinkBool  bool
	sinkFloat float64
	sinkStr   string
	sinkBuf   []byte
)

// benchOperands returns benchN numbers in scientific notation for a format
// of the given precision, as described above.
func benchOperands(full bool, prec int) (out [benchN]string) {
	r := rand.New(rand.NewPCG(42, uint64(prec)))
	for i := range out {
		nd := 1 + r.IntN(min(9, prec-1))
		frac := 2 + r.IntN(3)
		if full {
			nd = prec
			frac = nd - (1 + r.IntN(min(8, prec-3)))
		}
		b := make([]byte, 0, 48)
		if r.IntN(4) == 0 {
			b = append(b, '-')
		}
		b = append(b, byte('1'+r.IntN(9)))
		for j := 1; j < nd; j++ {
			b = append(b, byte('0'+r.IntN(10)))
		}
		b = append(b, 'E')
		out[i] = string(strconv.AppendInt(b, int64(-frac), 10))
	}
	return out
}
