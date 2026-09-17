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
  5–30 ns, and arithmetic, comparison, parsing and appending text never
  allocate. See [Performance](#performance).
- **Pure Go, no dependencies, no assembly, no cgo.** The hot paths are built
  on the `math/bits` intrinsics, which compile to single instructions.

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
x.Round(2)                              // to decimal places, never padding
x.RoundToIntegral(decimal.ToZero)

x.Equal(y); x.Less(y); x.Cmp(y)         // by value: 1.5 equals 1.50
x.CmpTotal(y)                           // by representation (IEEE totalOrder)

x.String()                              // "1.50"; round-trips exactly
fmt.Sprintf("%.3f|%8.2f|%e", x, x, x)   // fmt verbs, width, precision, flags
x.Int64(); x.Float64(); x.Decimal128()  // conversions
x.Bits(); x.DPD()                       // interchange encodings
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

### Equality, and why `==` does not compile

Decimal formats are redundant: `1`, `1.0` and `1.00` are distinct *members of
one cohort*, equal in value but with different exponents, and arithmetic
preserves those exponents (`1.20 + 1.1` is `2.30`). A bitwise `==` would say
they differ, so `Decimal64` and `Decimal128` are deliberately not comparable
and cannot be used as map keys directly. Use `Equal`/`Cmp` to compare values,
and `x.Reduce().Bits()` as a map key. (`Decimal32` is the exception: nothing
that forbids `==` fits in four bytes.)

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

Two behaviours are worth knowing because other implementations differ.
`Sqrt` honours the rounding direction, as IEEE 754 requires; implementations
of the General Decimal Arithmetic specification, such as decNumber and
Python's `decimal`, always round square roots half-even. And
`FMA(0, Inf, z)` raises Invalid even when `z` is a quiet NaN, a case the
standard leaves to the implementation.

Conformance is tested four ways:

1. **The General Decimal Arithmetic test cases** (`testdata/dectest`), Mike
   Cowlishaw's suite for exactly these formats: about 24,700 cases across the
   three formats run and pass. Cases are skipped only where they test
   something outside IEEE 754 (the logical and shift operations, `divideint`,
   engineering notation, rounding modes IEEE 754 does not have) or something
   specific to a DPD-in-memory implementation.
2. **A reference implementation** on `math/big` (`ref_test.go`), against which
   add, subtract, multiply, divide, fused multiply-add and square root are
   compared on hundreds of thousands of adversarially distributed random
   operands per format, in every rounding mode, flags included.
3. **Fuzzing** of the parsers and of arithmetic on arbitrary bit patterns,
   non-canonical encodings included, against the same reference.
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

| Operation          | `Decimal32` short | `Decimal32` full | `Decimal64` short | `Decimal64` full | `Decimal128` short | `Decimal128` full |
|--------------------|------------------:|-----------------:|------------------:|-----------------:|-------------------:|------------------:|
| `Add`              |               7.3 |               13 |               7.8 |               15 |                 12 |                33 |
| `Sub`              |               7.4 |               13 |               7.1 |               15 |                 12 |                33 |
| `Mul`              |               9.0 |               12 |               5.4 |               14 |                5.3 |                52 |
| `Quo`              |                17 |               14 |                21 |               21 |                 64 |                77 |
| `FMA`              |                48 |               47 |                41 |               54 |                 48 |                98 |
| `Sqrt`             |                22 |               21 |                32 |               31 |                165 |               152 |
| `Remainder`        |               8.1 |              7.9 |               7.8 |              7.9 |                 16 |                25 |
| `Quantize` to 0.01 |                12 |               13 |                12 |               14 |                 17 |                29 |
| `Round(2)`         |               8.2 |               12 |               7.9 |               12 |                 14 |                28 |
| `Cmp`              |               8.6 |              8.4 |               8.3 |              8.7 |                 12 |                13 |
| `Parse`            |                15 |               18 |                16 |               30 |                 16 |                65 |
| `String`           |            22 (1) |           25 (1) |            22 (1) |           30 (1) |             23 (1) |            49 (1) |
| `AppendText`       |                12 |               16 |                12 |               22 |                 13 |                41 |
| `Int64`            |                16 |               13 |                15 |               14 |                 15 |                23 |
| `Float64`          |               4.5 |              4.4 |               4.5 |               12 |                4.9 |           128 (1) |
| from `float64`     |                62 |               57 |                59 |               52 |                 74 |                65 |

#### Other libraries

Everyday amounts, up to 9 digits:

| Library                                                           |    Add |    Mul |      Quo |   Parse |  String |
|-------------------------------------------------------------------|-------:|-------:|---------:|--------:|--------:|
| **tendex/decimal** `Decimal64`                                    |    6.8 |    5.3 |       21 |      16 |  22 (1) |
| `float64` (binary)                                                |    0.5 |    0.5 |      0.5 |      25 |  45 (1) |
| [anz-bank/decimal](https://github.com/anz-bank/decimal)           |     19 |     15 |      8.3 | 148 (4) |  52 (1) |
| [govalues/decimal](https://github.com/govalues/decimal)           |    6.6 |    4.8 |  274 (1) |      26 |  19 (1) |
| [quagmt/udecimal](https://github.com/quagmt/udecimal)             |    8.2 |    5.7 |       17 |      13 |  28 (1) |
| [shopspring/decimal](https://github.com/shopspring/decimal)       | 60 (4) | 23 (2) | 172 (11) |  71 (3) | 110 (4) |
| [cockroachdb/apd](https://github.com/cockroachdb/apd)             |     31 |     30 |      100 |  82 (1) |  28 (1) |
| [ericlagergren/decimal](https://github.com/ericlagergren/decimal) |     18 |     11 |   54 (1) |  54 (1) |  71 (4) |

16 significant digits:

| Library                                                           |    Add |    Mul |      Quo |   Parse |  String |
|-------------------------------------------------------------------|-------:|-------:|---------:|--------:|--------:|
| **tendex/decimal** `Decimal64`                                    |     14 |     14 |       21 |      30 |  30 (1) |
| `float64` (binary)                                                |    0.5 |    0.5 |      0.5 |      50 |  45 (1) |
| [anz-bank/decimal](https://github.com/anz-bank/decimal)           |     18 |     17 |      8.1 | 187 (4) |  45 (1) |
| [govalues/decimal](https://github.com/govalues/decimal)           |     54 |    118 |  294 (2) |      45 |  31 (1) |
| [quagmt/udecimal](https://github.com/quagmt/udecimal)             |    8.7 |     12 |       16 |      23 |  47 (1) |
| [shopspring/decimal](https://github.com/shopspring/decimal)       | 89 (6) | 23 (2) | 186 (11) |  79 (3) | 100 (4) |
| [cockroachdb/apd](https://github.com/cockroachdb/apd)             |     98 |     95 |       91 |  95 (1) |  44 (2) |
| [ericlagergren/decimal](https://github.com/ericlagergren/decimal) |     44 | 67 (1) |   75 (2) |  73 (1) |  69 (5) |

34 significant digits:

| Library                                                           |     Add |     Mul |      Quo |   Parse |  String |
|-------------------------------------------------------------------|--------:|--------:|---------:|--------:|--------:|
| **tendex/decimal** `Decimal128`                                   |      33 |      52 |       76 |      66 |  50 (1) |
| [woodsbury/decimal128](https://github.com/woodsbury/decimal128)   |      24 |     106 |      269 |      45 | 185 (1) |
| [shopspring/decimal](https://github.com/shopspring/decimal)       |  86 (5) |  30 (2) | 329 (13) | 258 (5) | 141 (5) |
| [cockroachdb/apd](https://github.com/cockroachdb/apd)             | 169 (3) | 328 (7) |  296 (6) | 327 (4) | 136 (5) |
| [ericlagergren/decimal](https://github.com/ericlagergren/decimal) |      97 | 209 (3) |  208 (3) | 322 (4) | 146 (6) |

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
