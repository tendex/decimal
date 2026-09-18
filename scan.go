package decimal

import (
	"strconv"
	"strings"
)

// scanned is a decimal character sequence converted to an unrounded number:
// (-1)**neg × coef × 10**exp when finite, with sticky set if non-zero digits
// beyond the 38 most significant were discarded. For NaNs coef is the
// payload.
type scanned struct {
	coef   uint128
	exp    int
	neg    bool
	kind   kind
	sticky bool
}

// maxScanExp bounds the magnitude of a scanned exponent. Anything beyond it
// overflows or underflows every format, and clamping keeps exponent
// arithmetic far away from integer overflow.
const maxScanExp = 1 << 28

// scan converts s, a string or a byte slice, in the syntax accepted by the
// Parse functions:
//
//	[sign] digits [. digits] [e|E [sign] digits]
//	[sign] . digits [e|E [sign] digits]
//	[sign] inf | infinity
//	[sign] nan [digits] | snan [digits]
//
// Letters are matched without regard to case.
func scan[S string | []byte](s S) (r scanned, ok bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		r.neg = s[i] == '-'
		i++
	}
	if i == len(s) {
		return r, false
	}
	if c := s[i] | 0x20; c == 'i' || c == 'n' || c == 's' {
		return scanSpecial(r, s[i:])
	}

	// The first 19 significant digits accumulate in d[0], the next 19 in
	// d[1]; the rest only contribute to the exponent and the sticky bit.
	var d [2]uint64
	var n [2]int
	var frac, dropped int64
	sawDigit, sawDot := false, false
digits:
	for ; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '.':
			if sawDot {
				return r, false
			}
			sawDot = true
		case '0' <= ch && ch <= '9':
			sawDigit = true
			if sawDot {
				frac++
			}
			switch {
			case ch == '0' && n[0] == 0:
				// leading zero
			case n[0] < 19:
				d[0] = d[0]*10 + uint64(ch-'0')
				n[0]++
			case n[1] < 19:
				d[1] = d[1]*10 + uint64(ch-'0')
				n[1]++
			default:
				dropped++
				r.sticky = r.sticky || ch != '0'
			}
		default:
			break digits
		}
	}
	if !sawDigit {
		return r, false
	}

	var e int64
	if i < len(s) && s[i]|0x20 == 'e' {
		i++
		eneg := false
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			eneg = s[i] == '-'
			i++
		}
		if i == len(s) {
			return r, false
		}
		for ; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
			if e < maxScanExp {
				e = e*10 + int64(s[i]-'0')
			}
		}
		if eneg {
			e = -e
		}
	}
	if i != len(s) {
		return r, false
	}

	r.coef = pow10tab128[n[1]].mul64(d[0]).add64(d[1])
	r.exp = int(min(max(e-frac+dropped, -maxScanExp), maxScanExp))
	return r, true
}

// scanSpecial scans an infinity or NaN; the sign has been consumed. The
// conversions to string are of a few bytes, which the compiler keeps on the
// stack.
func scanSpecial[S string | []byte](r scanned, s S) (scanned, bool) {
	switch {
	case len(s) == 3 && strings.EqualFold(string(s), "inf"), len(s) == 8 && strings.EqualFold(string(s), "infinity"):
		r.kind = infinite
		return r, true
	case len(s) >= 4 && strings.EqualFold(string(s[:4]), "snan"):
		r.kind = signalingNaN
		s = s[4:]
	case len(s) >= 3 && strings.EqualFold(string(s[:3]), "nan"):
		r.kind = quietNaN
		s = s[3:]
	default:
		return r, false
	}
	// Optional payload.
	n := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || '9' < ch {
			return r, false
		}
		if n == 0 && ch == '0' {
			continue
		}
		if n++; n > 38 {
			return r, false
		}
		r.coef = r.coef.mul64(10).add64(uint64(ch - '0'))
	}
	return r, true
}

// parse converts s to format f, rounding if it has more digits than the
// format holds. parseBytes is the same for a byte slice, which the text
// unmarshalers and sql.Scanner receive, without converting it to a string.
func (c *Context) parse(f *format, fn, s string) (num, error) { return parseText(c, f, fn, s) }

func (c *Context) parseBytes(f *format, fn string, b []byte) (num, error) {
	return parseText(c, f, fn, b)
}

func parseText[S string | []byte](c *Context, f *format, fn string, s S) (num, error) {
	r, ok := scan(s)
	if ok && r.kind >= quietNaN && (r.coef.hi != 0 || r.coef.lo > f.maxPayload) {
		ok = false
	}
	if !ok {
		return num{kind: quietNaN}, syntaxError(fn, s)
	}
	if r.kind != finite {
		return num{coef: r.coef.lo, neg: r.neg, kind: r.kind}, nil
	}
	return c.round(f, r.neg, r.coef, r.exp, r.sticky), nil
}

func syntaxError[S string | []byte](fn string, s S) error {
	return &strconv.NumError{Func: "decimal." + fn, Num: strings.Clone(string(s)), Err: strconv.ErrSyntax}
}
