package decimal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// This file runs the General Decimal Arithmetic test cases ("decTest" files)
// by Mike Cowlishaw, the de facto conformance suite for IEEE 754 decimal
// arithmetic. testdata/dectest holds the files for the three fixed-size
// formats: ds* (decimal32), dd* (decimal64) and dq* (decimal128).

// A decOp runs one decTest operation and returns the result as a string in
// the form the test files use.
type decOp func(c *Context, args []string) (string, error)

// errSkip is returned by a decOp for a case that does not apply.
var errSkip = fmt.Errorf("skip")

// noHex skips cases with DPD-encoded operands. The quiet copy operations are
// tested on non-canonical DPD bit patterns, which they are expected to
// preserve; those patterns have no counterpart in the BID encoding this
// package stores.
func noHex(op decOp) decOp {
	return func(c *Context, args []string) (string, error) {
		for _, s := range args {
			if strings.HasPrefix(s, "#") {
				return "", errSkip
			}
		}
		return op(c, args)
	}
}

// decWantConds holds the expected conditions of the case being run, in
// lower case, for the few operations whose General Decimal Arithmetic
// definition is narrower than the IEEE 754 one.
var decWantConds string

// decClassNames maps classes to their decTest names.
var decClassNames = [...]string{
	SignalingNaN:      "sNaN",
	QuietNaN:          "NaN",
	NegativeInf:       "-Infinity",
	NegativeNormal:    "-Normal",
	NegativeSubnormal: "-Subnormal",
	NegativeZero:      "-Zero",
	PositiveZero:      "+Zero",
	PositiveSubnormal: "+Subnormal",
	PositiveNormal:    "+Normal",
	PositiveInf:       "+Infinity",
}

// decRounding maps the decTest rounding names to the modes of this package:
// the five of IEEE 754 and round-up. The other General Decimal Arithmetic
// modes (half_down, 05up) are not provided and those cases are skipped.
var decRounding = map[string]RoundingMode{
	"half_even": ToNearestEven,
	"half_up":   ToNearestAway,
	"down":      ToZero,
	"ceiling":   ToPositiveInf,
	"floor":     ToNegativeInf,
	"up":        AwayFromZero,
}

// decConditions maps decTest conditions to IEEE 754 flags. Clamped, Rounded,
// Subnormal and Lost_digits have no IEEE 754 equivalent.
var decConditions = map[string]Flags{
	"inexact":              Inexact,
	"overflow":             Overflow,
	"underflow":            Underflow,
	"division_by_zero":     DivisionByZero,
	"invalid_operation":    Invalid,
	"division_impossible":  Invalid,
	"division_undefined":   Invalid,
	"conversion_syntax":    Invalid,
	"clamped":              0,
	"rounded":              0,
	"subnormal":            0,
	"lost_digits":          0,
	"insufficient_storage": 0,
}

// decFields splits a test line into tokens, honouring quotes.
func decFields(line string) []string {
	var out []string
	for i := 0; i < len(line); {
		switch c := line[i]; {
		case c == ' ' || c == '\t':
			i++
		case c == '\'' || c == '"':
			var b strings.Builder
			for i++; i < len(line); i++ {
				if line[i] == c {
					if i+1 < len(line) && line[i+1] == c {
						i++ // doubled quote
					} else {
						break
					}
				}
				b.WriteByte(line[i])
			}
			i++
			out = append(out, b.String())
		case c == '-' && strings.HasPrefix(line[i:], "--"):
			return out
		default:
			j := i
			for j < len(line) && line[j] != ' ' && line[j] != '\t' {
				j++
			}
			out = append(out, line[i:j])
			i = j
		}
	}
	return out
}

type decStats struct {
	run     int
	skipped map[string]int
}

// runDecTests runs every file matching pattern against ops.
func runDecTests(t *testing.T, pattern string, ops map[string]decOp, hex func(string) string) {
	files, err := filepath.Glob(filepath.Join("testdata", "dectest", pattern))
	if err != nil || len(files) == 0 {
		t.Fatalf("no test files match %q (%v)", pattern, err)
	}
	stats := decStats{skipped: map[string]int{}}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".decTest")
		t.Run(name, func(t *testing.T) { runDecFile(t, file, ops, hex, &stats) })
	}
	var skipped []string
	for op, n := range stats.skipped {
		skipped = append(skipped, fmt.Sprintf("%s:%d", op, n))
	}
	sort.Strings(skipped)
	t.Logf("%d cases run; skipped %v", stats.run, skipped)
}

func runDecFile(t *testing.T, file string, ops map[string]decOp, hex func(string) string, stats *decStats) {
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var mode RoundingMode
	modeOK := true
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		tok := decFields(sc.Text())
		if len(tok) == 0 {
			continue
		}
		if strings.HasSuffix(tok[0], ":") {
			if strings.EqualFold(tok[0], "rounding:") && len(tok) > 1 {
				mode, modeOK = decRounding[strings.ToLower(tok[1])]
			}
			continue
		}
		arrow := -1
		for i, s := range tok {
			if s == "->" {
				arrow = i
			}
		}
		if arrow < 2 || arrow+1 >= len(tok) {
			continue
		}
		id, opName, args, want := tok[0], strings.ToLower(tok[1]), tok[2:arrow], tok[arrow+1]
		op := ops[opName]
		if op == nil {
			stats.skipped[opName]++
			continue
		}
		if !modeOK {
			stats.skipped["(rounding)"]++
			continue
		}
		var wantFlags Flags
		for _, cond := range tok[arrow+2:] {
			fl, ok := decConditions[strings.ToLower(cond)]
			if !ok {
				t.Fatalf("%s: unknown condition %q", id, cond)
			}
			wantFlags |= fl
		}

		decWantConds = strings.ToLower(strings.Join(tok[arrow+2:], " "))
		c := Context{Rounding: mode}
		got, err := op(&c, args)
		if err == errSkip {
			stats.skipped[opName]++
			continue
		}
		if err != nil {
			t.Errorf("%s: %s %v: %v", id, opName, args, err)
			continue
		}
		stats.run++
		if want == "?" {
			want = got // result undefined; only the flags are checked
		}
		if strings.HasPrefix(want, "#") {
			// The expected result is a DPD encoding in hexadecimal.
			want = strings.ToLower(want)
			got = hex(got)
		}
		if got != want || c.Flags != wantFlags {
			t.Errorf("%s: %s %v (%v)\n\t got %s [%v]\n\twant %s [%v]",
				id, opName, args, mode, got, c.Flags, want, wantFlags)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}
