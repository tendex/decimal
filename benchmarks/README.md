# Benchmarks

This module times [`github.com/tendex/decimal`](..) and compares it with
`float64` and with other Go decimal libraries. It is a separate module so that
the decimal module itself keeps no dependencies. The tables in the
[main README](../README.md#performance) are generated from the results here.

## Running

```sh
./run.sh
```

runs the benchmarks of the decimal package and of this module in interleaved
rounds, so that drift in the speed of the machine affects them all alike. It
writes the raw results to [`results/`](results) and regenerates the tables in
the main README with [`cmd/benchtable`](cmd/benchtable). The defaults, 6
rounds of at least 250ms per benchmark, take about seven minutes on an Apple
M3; set `ROUNDS` and `BENCHTIME` to change them.

Quit other programs first, keep a laptop on power, and run the script in the
foreground of a terminal. macOS may run a process started in the background,
or by a tool running in the background, at low priority on the efficiency
cores, where results are several times slower and far noisier. `benchtable`
refuses to update the README when more than a tenth of the benchmarks vary by
over 15% between runs (`-force` overrides this).

To see whether a change made a difference, keep the old results and compare
them with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```sh
cp -r results old
./run.sh
benchstat old/decimal.txt results/decimal.txt
```

Individual benchmarks run as usual:

```sh
go test -run '^$' -bench 'Digits16/Quo' .                   # one operation, every library
go test -run '^$' -bench 'Decimal128/Mul' -benchmem ..      # the decimal package's own
```

## What is measured

- `BenchmarkDecimal32`, `BenchmarkDecimal64` and `BenchmarkDecimal128`, in the
  decimal package, time sixteen operations on Short and Full operands.
- `BenchmarkAmounts`, `BenchmarkDigits16` and `BenchmarkDigits34`, in this
  module, time `Add`, `Mul`, `Quo`, `Parse` and `String` in each library, on
  the Short operands of `Decimal64` and the Full operands of `Decimal64` and
  `Decimal128`, written without exponents.

Short operands have up to nine significant digits, two to four of them after
the decimal point. Full operands use every digit of the format, with one to
eight before the decimal point, so that most results must be rounded.

Each benchmark cycles through 1,024 operands, so that branch prediction sees a
realistic mix rather than a single path. Each loop calls the library directly,
as user code would, so that the compiler can inline the call, and stores the
result in a package-level variable so that it is not optimized away. Libraries
whose APIs write into a destination value (apd, ericlagergren) reuse one, as
their documentation intends.

## Reading the comparison

The libraries do not all compute the same thing:

| Library               | Representation                         | Sums and products                           | Quotients                  |
|-----------------------|----------------------------------------|---------------------------------------------|----------------------------|
| tendex/decimal        | IEEE 754 decimal64 or decimal128       | rounded to 16 or 34 digits                  | rounded to 16 or 34 digits |
| anz-bank/decimal      | IEEE 754 decimal64                     | rounded to 16 digits                        | rounded to 16 digits       |
| woodsbury/decimal128  | 128-bit decimal                        | rounded to 34 digits                        | rounded to 34 digits       |
| govalues/decimal      | fixed point, 19-digit coefficient      | rounded to fit                              | rounded to fit             |
| quagmt/udecimal       | fixed point, 19 digits after the point | exact sums; products truncated to 19 places | truncated to 19 places     |
| shopspring/decimal    | arbitrary precision                    | exact                                       | rounded to 16 or 34 places |
| cockroachdb/apd       | arbitrary precision                    | rounded to 16 or 34 digits                  | rounded to 16 or 34 digits |
| ericlagergren/decimal | arbitrary precision                    | rounded to 16 or 34 digits                  | rounded to 16 or 34 digits |
| `float64`             | binary floating point                  | rounded in binary                           | rounded in binary          |

govalues and udecimal cannot hold 34 digits, and ANZ implements only decimal64,
so they appear only in the 16-digit comparisons. `float64` is not a decimal
type and cannot represent most of the operands exactly; it is included to show
what exact decimal arithmetic costs.

`TestAgreement` checks that every library computes, without error, what its
benchmarks claim to measure: each result must match the exact result, or the
exact result rounded to 16 or 34 digits, to within one unit in the last place
(for `float64`, to nine digits). It guards against measuring error paths or
misconfigured contexts; it is not a conformance test. CI runs it, along with
one iteration of every benchmark.

Library versions are pinned in [`go.mod`](go.mod).

## Adding a library

Add a file like the existing ones, with a function that registers the
`Add`, `Mul`, `Quo`, `Parse` and `String` sub-benchmarks and one that returns
the same results as strings. List the library in `libraries16` or
`libraries34` in [`compare_test.go`](compare_test.go), and in `workloads` and
`libraryLabels` in [`cmd/benchtable`](cmd/benchtable/main.go).
