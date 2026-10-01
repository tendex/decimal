# decimal

Package `decimal` implements IEEE 754 decimal floating-point arithmetic for
Go: the fixed-size formats decimal32, decimal64 and decimal128, as the value
types `Decimal32`, `Decimal64` and `Decimal128`.

```go
import "github.com/tendex/decimal"

price := decimal.MustParse64("19.99")
qty := decimal.New64(3, 0)

subtotal := price.Mul(qty)                                       // 59.97, exactly
tax := subtotal.Mul(decimal.MustParse64("0.0825")).Round(2)      // 4.95
fmt.Println(subtotal.Add(tax))                                   // 64.92
```

- **Conformant.** Every operation IEEE 754-2019 requires of a decimal format,
  correctly rounded in all five rounding directions, with the five exception
  flags, subnormals, signed zeros, infinities, quiet and signaling NaNs with
  payloads, cohorts and preferred exponents, and both interchange encodings
  (BID and DPD).
- **Fast.** Plain 4-, 8- and 16-byte values; most `Decimal64` operations take
  5–30 ns, and of the operations measured below only `String` allocates, for
  the string it returns. See [Performance](#performance).
- **Pure Go, no dependencies, no assembly, no cgo.** The hot paths are built
  on the `math/bits` intrinsics, which compile to single instructions.

## Installation

```sh
go get github.com/tendex/decimal@v0.1.0
```

Go 1.24 or later is required. During the v0 series, the API may change between
releases.

## Usage

The three types have identical method sets. `Decimal64` (16 digits) is the
natural choice for most purposes; `Decimal128` (34 digits) when 16 are not
enough; `Decimal32` (7 digits) for compact storage.

```go
x, err := decimal.Parse64("1.50")       // from text; the exponent is kept
y := decimal.New64(25, -1)              // 25 × 10**-1 = 2.5
z := decimal.New64FromFloat(0.5)        // from binary floating point

x.Add(y); x.Sub(y); x.Mul(y); x.Quo(y)  // correctly rounded, ties to even
x.FMA(y, z); x.Sqrt(); x.Remainder(y); x.Mod(y)
x.Quantize(decimal.MustParse64("0.01")) // to a given exponent, padding
x.RoundToMultiple(y)                    // to a multiple of y, such as a tick size
x.Round(2)                              // to decimal places, never padding
x.RoundToIntegral(decimal.ToZero)

x.Equal(y); x.Less(y); x.Cmp(y)         // by value: 1.5 equals 1.50
x.CmpTotal(y)                           // by representation (IEEE totalOrder)

x.String()                              // "1.50"; round-trips exactly
fmt.Sprintf("%.3f|%8.2f|%e", x, x, x)   // fmt verbs, width, precision, flags
x.Int64(); x.Float64(); x.Decimal128()  // conversions
x.Bits(); x.DPD()                       // interchange encodings
x.Value(); x.Scan(v)                    // database/sql; sql.Null[Decimal64] for NULL
```

The zero value of each type is the number 0, so values can be declared and
accumulated into without initialization.

### Rounding modes and exception flags

The methods above round to nearest, ties to even, and return the IEEE 754
default results for exceptional cases (`1/0` is an infinity, `0/0` a NaN). A
`Context` supplies the other rounding directions and records exceptions as
sticky flags:

```go
c := decimal.Context{Rounding: decimal.ToZero}
q := c.Quo64(x, y)
if c.Flags&(decimal.Inexact|decimal.Overflow) != 0 {
	// ...
}
```

`Context` methods are named after the operation and the width of the format:
`Add64`, `Sqrt128`, `Quantize32`; conversions out of a format are
`<Destination>From<Width>`, as in `Int64From64`. A `Context` is two bytes of
plain data; there is no global or per-goroutine state.

A sixth direction, `AwayFromZero`, is the round-up of the General Decimal
Arithmetic specification, which IEEE 754 lacks and which fees and margins
often call for.

### Equality, and why `==` does not compile

Decimal formats are redundant: `1`, `1.0` and `1.00` are distinct *members of
one cohort*, equal in value but with different exponents, and arithmetic
preserves those exponents (`1.20 + 1.1` is `2.30`). A bitwise `==` would say
they differ, so `Decimal64` and `Decimal128` are deliberately not comparable
and cannot be used as map keys directly. (`Decimal32` is the exception:
nothing that forbids `==` fits in four bytes.) Use `Equal`/`Cmp` to compare
values. For a map key by value, reduce first, and also give zeros one sign:
`+0` and `-0` are equal, but `Reduce` keeps the sign, so their bits differ.

```go
func key(x decimal.Decimal64) uint64 {
	if x.IsZero() {
		x = decimal.Decimal64{} // -0 and +0 are equal; make them one key
	}
	return x.Reduce().Bits()
}
```

## Conformance

| IEEE 754-2019                                                                                            | Provided as                                                                                                |
|----------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------|
| addition, subtraction, multiplication, division, fusedMultiplyAdd, squareRoot                            | `Add` `Sub` `Mul` `Quo` `FMA` `Sqrt`                                                                       |
| remainder                                                                                                | `Remainder` (and `Mod`, truncating like `math.Mod`)                                                        |
| roundToIntegral{TiesToEven,…}, roundToIntegralExact                                                      | `RoundToIntegral(mode)`, `Context.RoundToIntegralExact64`                                                  |
| nextUp, nextDown                                                                                         | `NextUp` `NextDown`                                                                                        |
| quantize, quantum, sameQuantum                                                                           | `Quantize` `Quantum` `SameQuantum`                                                                         |
| logB, scaleB                                                                                             | `LogB` `ScaleB`                                                                                            |
| minimum, maximum, minimumNumber, maximumNumber                                                           | `Min` `Max`, `Context.MinNum64` `Context.MaxNum64`                                                         |
| convertFromInt, convertToInteger…, convertToIntegerExact…                                                | `New64` `New64FromUint`, `Int64` `Uint64`, `Context.Int64From64` …                                         |
| convertFormat                                                                                            | `Decimal32()` `Decimal64()` `Decimal128()`, `New64FromFloat` `Float64`, and their `Context` forms          |
| convertFromDecimalCharacter, convertToDecimalCharacter                                                   | `Parse64`, `String` `Text` `Append` `Format`                                                               |
| copy, negate, abs, copySign                                                                              | assignment, `Neg` `Abs` `CopySign`                                                                         |
| compareQuiet…, compareSignaling…                                                                         | `Compare` `Equal` `Less`, `Context.Compare64` `Context.CompareSignal64`                                    |
| totalOrder, totalOrderMag                                                                                | `CmpTotal`, `x.Abs().CmpTotal(y.Abs())`                                                                    |
| class, isSignMinus, isNormal, isFinite, isZero, isSubnormal, isInfinite, isNaN, isSignaling, isCanonical | `Class` `Signbit` `IsNormal` `IsFinite` `IsZero` `IsSubnormal` `IsInf` `IsNaN` `IsSignaling` `IsCanonical` |
| encodeDecimal, decodeDecimal (DPD), BID                                                                  | `DPD` `New64FromDPD`, `Bits` `New64FromBits`                                                               |
| rounding-direction attribute; lower/raise/test/save/restore flags                                        | the `Rounding` and `Flags` fields of `Context`                                                             |

Not provided: alternate exception handling (traps), and the transcendental
functions of clause 9, both of which the standard makes optional.

Provided beyond the standard: `Round` to a number of decimal places, `Mod`,
`RoundToMultiple` to a multiple of any increment, and the `AwayFromZero`
rounding direction.

Two behaviours are worth knowing because other implementations differ.
`Sqrt` honours the rounding direction, as IEEE 754 requires; implementations
of the General Decimal Arithmetic specification, such as decNumber and
Python's `decimal`, always round square roots half-even. And
`FMA(0, Inf, z)` raises Invalid even when `z` is a quiet NaN, a case the
standard leaves to the implementation.

Conformance is tested four ways:

1. **The General Decimal Arithmetic test cases** (`testdata/dectest`), Mike
   Cowlishaw's suite for exactly these formats: about 25,000 cases across the
   three formats run and pass. Cases are skipped only where they test
   something outside IEEE 754 (the logical and shift operations, `divideint`,
   engineering notation, the half_down and 05up rounding modes) or something
   specific to a DPD-in-memory implementation.
2. **A reference implementation** on `math/big` (`ref_test.go`), against which
   add, subtract, multiply, divide, fused multiply-add and square root are
   compared on hundreds of thousands of adversarially distributed random
   operands per format, in every rounding mode, flags included.
3. **Fuzzing.** The parsers are checked for panics and exact round-trips
   through `String`. Arithmetic on arbitrary bit patterns, non-canonical
   encodings included, is checked against the same reference.
4. **An independent implementation.** `internal/xcheck` prints random
   operations (fourteen kinds, both large formats, all rounding modes, flags
   included) for Python's `decimal` module (libmpdec) to verify. Several
   million have been checked without a mismatch.

```
go test ./...                                 # everything; under a minute
go test -short ./...                          # fewer random cases; about 3 s
go test -run '^$' -fuzz FuzzArithmetic64 .    # fuzz
go run ./internal/xcheck 100000 | python3 internal/xcheck/verify.py
```

CI runs a bounded independent cross-check on every change. A weekly workflow
fuzzes all four targets for ten minutes each; it can also be run manually, and
retains failing inputs as downloadable artifacts.

## Performance

The tables below come from the [benchmarks module](benchmarks), which times
this package and compares it with `float64` and with other Go decimal
libraries on the same operands.

"Short" operands, like the everyday amounts in the comparison, have up to nine
significant digits, two to four of them after the decimal point, such as
`-1234.56` (up to six digits for `Decimal32`). "Full" operands use every digit
of the format, with one to eight before the decimal point (one to four for
`Decimal32`), so that most results must be rounded.

<!-- benchmarks -->

Measured on Apple M3 (darwin/arm64) with go1.27.1: the median of 6 runs, each of at least 250ms.
Times are in nanoseconds per operation; allocations per operation follow in
parentheses where there are any.

#### This package

| Operation                 | `Decimal32` short | `Decimal32` full | `Decimal64` short | `Decimal64` full | `Decimal128` short | `Decimal128` full |
|---------------------------|------------------:|-----------------:|------------------:|-----------------:|-------------------:|------------------:|
| `Add`                     |               7.1 |               13 |               6.8 |               14 |                 12 |                33 |
| `Sub`                     |               7.2 |               13 |               6.9 |               15 |                 13 |                33 |
| `Mul`                     |               8.9 |               11 |               5.4 |               14 |                5.3 |                52 |
| `Quo`                     |                17 |               14 |                21 |               22 |                 64 |                77 |
| `FMA`                     |                47 |               47 |                41 |               55 |                 49 |                98 |
| `Sqrt`                    |                22 |               21 |                32 |               31 |                167 |               153 |
| `Remainder`               |               7.8 |              7.7 |               7.6 |              7.9 |                 16 |                25 |
| `Quantize` to 0.01        |                12 |               13 |                12 |               14 |                 18 |                29 |
| `RoundToMultiple` to 0.05 |               7.6 |               11 |               7.3 |               10 |                 28 |                39 |
| `Round(2)`                |               8.5 |               12 |               7.8 |               12 |                 14 |                28 |
| `Cmp`                     |               8.7 |              8.5 |               8.5 |              9.0 |                 12 |                13 |
| `Parse`                   |                15 |               19 |                17 |               30 |                 16 |                66 |
| `String`                  |            23 (1) |           25 (1) |            23 (1) |           32 (1) |             23 (1) |            52 (1) |
| `AppendText`              |                12 |               16 |                12 |               22 |                 13 |                41 |
| `Int64`                   |                16 |               14 |                16 |               14 |                 16 |                24 |
| `Float64`                 |               4.7 |              4.5 |               4.5 |              5.2 |                5.1 |                14 |
| from `float64`            |                63 |               58 |                59 |               52 |                 73 |                65 |

#### Other libraries

Everyday amounts, up to 9 digits:

| Library                                                           |    Add |    Mul |      Quo |   Parse |  String |
|-------------------------------------------------------------------|-------:|-------:|---------:|--------:|--------:|
| **tendex/decimal** `Decimal64`                                    |    6.8 |    5.4 |       21 |      17 |  23 (1) |
| `float64` (binary)                                                |    0.6 |    0.5 |      0.6 |      25 |  49 (1) |
| [anz-bank/decimal](https://github.com/anz-bank/decimal)           |     20 |     15 |      8.6 | 154 (4) |  55 (1) |
| [govalues/decimal](https://github.com/govalues/decimal)           |    6.8 |    4.8 |  275 (1) |      26 |  19 (1) |
| [quagmt/udecimal](https://github.com/quagmt/udecimal)             |    8.3 |    5.8 |       17 |      14 |  28 (1) |
| [shopspring/decimal](https://github.com/shopspring/decimal)       | 60 (4) | 23 (2) | 174 (11) |  71 (3) | 110 (4) |
| [cockroachdb/apd](https://github.com/cockroachdb/apd)             |     31 |     30 |      104 |  83 (1) |  28 (1) |
| [ericlagergren/decimal](https://github.com/ericlagergren/decimal) |     18 |     11 |   55 (1) |  55 (1) |  70 (4) |

16 significant digits:

| Library                                                           |    Add |    Mul |      Quo |   Parse |  String |
|-------------------------------------------------------------------|-------:|-------:|---------:|--------:|--------:|
| **tendex/decimal** `Decimal64`                                    |     14 |     14 |       21 |      31 |  30 (1) |
| `float64` (binary)                                                |    0.5 |    0.5 |      0.5 |      50 |  45 (1) |
| [anz-bank/decimal](https://github.com/anz-bank/decimal)           |     18 |     17 |      8.1 | 189 (4) |  47 (1) |
| [govalues/decimal](https://github.com/govalues/decimal)           |     54 |    119 |  297 (2) |      45 |  31 (1) |
| [quagmt/udecimal](https://github.com/quagmt/udecimal)             |    8.8 |     12 |       16 |      27 |  47 (1) |
| [shopspring/decimal](https://github.com/shopspring/decimal)       | 89 (6) | 23 (2) | 186 (11) |  78 (3) | 100 (4) |
| [cockroachdb/apd](https://github.com/cockroachdb/apd)             |     99 |     95 |       91 |  95 (1) |  44 (2) |
| [ericlagergren/decimal](https://github.com/ericlagergren/decimal) |     44 | 67 (1) |   75 (2) |  73 (1) |  69 (5) |

34 significant digits:

| Library                                                           |     Add |     Mul |      Quo |   Parse |  String |
|-------------------------------------------------------------------|--------:|--------:|---------:|--------:|--------:|
| **tendex/decimal** `Decimal128`                                   |      33 |      52 |       78 |      67 |  50 (1) |
| [woodsbury/decimal128](https://github.com/woodsbury/decimal128)   |      24 |     105 |      271 |      45 | 184 (1) |
| [shopspring/decimal](https://github.com/shopspring/decimal)       |  86 (5) |  30 (2) | 328 (13) | 260 (5) | 141 (5) |
| [cockroachdb/apd](https://github.com/cockroachdb/apd)             | 169 (3) | 328 (7) |  297 (6) | 328 (4) | 136 (5) |
| [ericlagergren/decimal](https://github.com/ericlagergren/decimal) |      98 | 210 (3) |  209 (3) | 319 (4) | 147 (6) |

<!-- /benchmarks -->

Each library is used as its API intends, on the same operands, but they do not
all compute the same thing: some round to significant digits, some to decimal
places, some truncate, and some keep every digit, while `float64` cannot
represent most of the operands at all. The fixed-point libraries hold at most
19 digits, so they appear only at 16. A test checks that every library's
results agree with the exact ones to within one unit in the last place, so no
benchmark measures an error path. See
[benchmarks/README.md](benchmarks/README.md) for the differences between the
libraries and for how to reproduce these numbers with `benchmarks/run.sh`.

## Design

- Values hold the IEEE 754 BID encoding, XORed with the encoding of `+0E+0`
  so that the Go zero value is a well-behaved zero rather than `0E-398`.
  `Bits` and `FromBits` undo the XOR; nothing else can observe it.
- Operations unpack to sign, coefficient and exponent, compute in integers
  with a sticky bit, and round once while packing, in the manner of
  `runtime/softfloat64.go`. `Decimal32` and `Decimal64` share a kernel on
  `uint64` coefficients with 128-bit intermediates (`num*.go`); `Decimal128`
  has its own on 128-bit coefficients with 256-bit intermediates
  (`num128*.go`).
- Rounding divides by powers of ten using precomputed reciprocals (Möller and
  Granlund) rather than division instructions, and the data-dependent steps
  of rounding are branch-free.
- Conversion to `float64` multiplies the coefficient by a 128-bit
  approximation of the power of ten, after Eisel and Lemire, and rounds the
  product (`float.go`). The approximation is inexact, so the conversion
  computes the interval the exact value lies in and uses the result only when
  both ends round to the same `float64`; for the roughly one value in a
  hundred that lands too close to halfway, it writes the digits out and lets
  `strconv` decide.
- There is no assembly. Go's targets other than POWER and IBM Z have no
  decimal instructions; what the kernels need from the hardware (wide
  multiply, add with carry, count leading zeros) the compiler already provides
  through `math/bits`, inlined, which an assembly function could not be.
- The `Decimal32` and `Decimal128` method sets and test adapters are generated
  from the `Decimal64` ones by `gen_formats.go`; lookup tables by
  `gen_tables.go`. Run `go generate` after editing a `decimal64_*.go` file.

## Licence

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE); the latter records the
provenance of the vendored conformance test cases in `testdata/dectest`, which
are © IBM Corporation and distributed unmodified under the ICU License (see
[testdata/dectest/LICENSE](testdata/dectest/LICENSE)). They are used only by the
tests and are not part of the compiled package.
