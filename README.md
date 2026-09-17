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
- **Fast.** Plain 4-, 8- and 16-byte values; arithmetic in 6–25 ns for
  `Decimal64`; arithmetic, comparison, parsing and appending text never
  allocate.
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

`go test -bench .` on an Apple M3, Go 1.27. "Short" operands are everyday
amounts of up to nine digits; "Full" operands use the whole precision, so
every result needs rounding.

| ns/op | Decimal64 short | Decimal64 full | Decimal128 short | Decimal128 full |
|-------|----------------:|---------------:|-----------------:|----------------:|
| Add   |               8 |             16 |               13 |              37 |
| Mul   |               6 |             15 |                6 |              52 |
| Quo   |              22 |             26 |               64 |              78 |
| FMA   |              38 |             54 |               47 |              95 |
| Sqrt  |              34 |             33 |              164 |             151 |
| Cmp   |              10 |             10 |               13 |              15 |

Parsing takes 15–30 ns (`Decimal64`) and formatting with `AppendText` 20 ns,
both without allocating.

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

## Test data

`testdata/dectest` contains the decimal32/64/128 files of the General Decimal
Arithmetic test cases, © IBM Corporation, which carry their own notices. Check
their terms (the ICU licence) before publishing this repository.
