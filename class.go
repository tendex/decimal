package decimal

import "strconv"

// Class is one of the ten IEEE 754 classes every floating-point datum falls
// into.
type Class uint8

// The IEEE 754 classes, in the order the standard lists them.
const (
	SignalingNaN Class = iota
	QuietNaN
	NegativeInf
	NegativeNormal
	NegativeSubnormal
	NegativeZero
	PositiveZero
	PositiveSubnormal
	PositiveNormal
	PositiveInf
)

var classNames = [...]string{
	"signalingNaN",
	"quietNaN",
	"negativeInfinity",
	"negativeNormal",
	"negativeSubnormal",
	"negativeZero",
	"positiveZero",
	"positiveSubnormal",
	"positiveNormal",
	"positiveInfinity",
}

// String returns the name IEEE 754 gives the class, such as
// "negativeSubnormal".
func (c Class) String() string {
	if int(c) < len(classNames) {
		return classNames[c]
	}
	return "Class(" + strconv.Itoa(int(c)) + ")"
}

// Ordering is the result of comparing two decimal numbers. Exactly one of
// the four mutually exclusive IEEE 754 relations holds between any two
// values, so every comparison predicate of the standard is a test on an
// Ordering.
type Ordering int8

// The four IEEE 754 relations.
const (
	Less      Ordering = -1
	Equal     Ordering = 0
	Greater   Ordering = 1
	Unordered Ordering = 2 // at least one operand is a NaN
)

// String returns the name of the relation.
func (o Ordering) String() string {
	switch o {
	case Less:
		return "Less"
	case Equal:
		return "Equal"
	case Greater:
		return "Greater"
	case Unordered:
		return "Unordered"
	}
	return "Ordering(" + strconv.Itoa(int(o)) + ")"
}
