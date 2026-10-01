package decimal

import (
	"strconv"
	"strings"
)

// RoundingMode is a rounding direction: one of the five IEEE 754
// rounding-direction attributes, or AwayFromZero. The names follow math/big.
type RoundingMode uint8

// The five IEEE 754 rounding directions, and AwayFromZero, the round-up of
// the General Decimal Arithmetic specification, which IEEE 754 lacks. The
// zero value, ToNearestEven, is the IEEE 754 default.
const (
	ToNearestEven RoundingMode = iota // roundTiesToEven
	ToNearestAway                     // roundTiesToAway
	ToZero                            // roundTowardZero
	ToPositiveInf                     // roundTowardPositive
	ToNegativeInf                     // roundTowardNegative
	AwayFromZero                      // round-up: away from zero; not IEEE 754
)

// String returns the name of the rounding mode.
func (m RoundingMode) String() string {
	switch m {
	case ToNearestEven:
		return "ToNearestEven"
	case ToNearestAway:
		return "ToNearestAway"
	case ToZero:
		return "ToZero"
	case ToPositiveInf:
		return "ToPositiveInf"
	case ToNegativeInf:
		return "ToNegativeInf"
	case AwayFromZero:
		return "AwayFromZero"
	}
	return "RoundingMode(" + strconv.Itoa(int(m)) + ")"
}

// Flags is a set of IEEE 754 exception status flags.
type Flags uint8

// The five IEEE 754 exceptions.
const (
	// Invalid is raised when an operation has no usefully definable result,
	// such as 0/0, Inf-Inf, sqrt(-1), or any arithmetic on a signaling NaN.
	Invalid Flags = 1 << iota
	// DivisionByZero is raised when an exact infinite result is produced
	// from finite operands, such as 1/0 or logB(0).
	DivisionByZero
	// Overflow is raised when the rounded result would exceed the largest
	// finite number of the format.
	Overflow
	// Underflow is raised when a result is both tiny (non-zero and smaller
	// in magnitude than the smallest normal number) and inexact.
	Underflow
	// Inexact is raised when the rounded result differs from the exact
	// result.
	Inexact
)

var flagNames = [...]string{"Invalid", "DivisionByZero", "Overflow", "Underflow", "Inexact"}

// String returns the names of the flags in f separated by "|", or "0" if no
// flag is set.
func (f Flags) String() string {
	if f == 0 {
		return "0"
	}
	var names []string
	for i, name := range flagNames {
		if f&(1<<i) != 0 {
			names = append(names, name)
		}
	}
	if rest := f >> len(flagNames) << len(flagNames); rest != 0 {
		names = append(names, "Flags("+strconv.Itoa(int(rest))+")")
	}
	return strings.Join(names, "|")
}

// A Context carries the dynamic attributes of IEEE 754 arithmetic: the
// rounding direction in effect and the sticky exception flags raised so far.
//
// The zero Context rounds to nearest, ties to even, and has no flags set. A
// Context is plain data: it may be copied, compared and reset freely
// (saving, restoring and lowering flags are ordinary assignments), but a
// single Context must not be used from multiple goroutines concurrently.
//
// Context methods are named after the operation followed by the width of the
// decimal format they operate on: Add64 adds two Decimal64 values, Sqrt128
// takes the square root of a Decimal128. Conversions out of a decimal format
// are named <Destination>From<Width>.
//
// The arithmetic methods on the decimal types themselves, such as
// Decimal64.Add, behave as the corresponding Context method on a zero
// Context whose flags are discarded.
type Context struct {
	// Rounding is the rounding direction used when a result is inexact.
	Rounding RoundingMode
	// Flags accumulates the exceptions raised by operations on the
	// Context. Operations only ever set flags; clearing them is up to the
	// caller.
	Flags Flags
}

// Raised reports whether any of the given flags is set in c.
func (c *Context) Raised(f Flags) bool { return c.Flags&f != 0 }
