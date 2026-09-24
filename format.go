package decimal

import (
	"fmt"
	"strconv"
)

// A text is a decimal number unpacked for conversion to characters. The
// coefficient is held as ASCII digits without leading zeros (none at all for
// a zero coefficient), so that value = 0.d[0]d[1]... × 10**dp. For NaNs the
// digits are the payload.
//
// The digits are located by offset rather than with a slice so that a text
// does not point into itself, which would force it onto the heap.
type text struct {
	buf  [40]byte
	off  int // the digits are buf[off : off+nd]
	nd   int
	dp   int // decimal point position
	exp  int // the original exponent q
	neg  bool
	kind kind
}

var zeroDigit = [...]byte{'0'}

func (t *text) digits() []byte { return t.buf[t.off : t.off+t.nd] }

// setCoef fills in the digits of a coefficient or payload and the exponent.
func (t *text) setCoef(coef uint128, exp int) {
	i := len(t.buf)
	for coef.hi != 0 {
		var r uint64
		coef, r = coef.quoRemPow10(19)
		for j := 0; j < 19; j++ {
			i--
			t.buf[i] = byte('0' + r%10)
			r /= 10
		}
	}
	for v := coef.lo; v != 0; v /= 10 {
		i--
		t.buf[i] = byte('0' + v%10)
	}
	// Splitting at 10**19 can leave leading zeros in the low chunk only
	// when a higher chunk follows, so none remain at the front.
	t.off, t.nd = i, len(t.buf)-i
	t.exp = exp
	t.dp = t.nd + exp
}

// appendSpecial appends an infinity or NaN and reports whether t is one.
func (t *text) appendSpecial(b []byte) ([]byte, bool) {
	switch t.kind {
	case finite:
		return b, false
	case infinite:
		b = append(b, "Infinity"...)
	case signalingNaN:
		b = append(b, "sNaN"...)
		b = append(b, t.digits()...)
	default:
		b = append(b, "NaN"...)
		b = append(b, t.digits()...)
	}
	return b, true
}

// appendSci appends the IEEE 754 / General Decimal Arithmetic scientific
// string, which preserves the exponent and therefore round-trips exactly.
func (t *text) appendSci(b []byte) []byte {
	if t.neg {
		b = append(b, '-')
	}
	if b, ok := t.appendSpecial(b); ok {
		return b
	}
	d := t.digits()
	if len(d) == 0 {
		d = zeroDigit[:]
	}
	adj := t.exp + len(d) - 1
	switch {
	case t.exp == 0:
		return append(b, d...)
	case t.exp < 0 && adj >= -6:
		if pt := len(d) + t.exp; pt > 0 {
			b = append(b, d[:pt]...)
			b = append(b, '.')
			return append(b, d[pt:]...)
		} else {
			b = append(b, '0', '.')
			for ; pt < 0; pt++ {
				b = append(b, '0')
			}
			return append(b, d...)
		}
	}
	b = append(b, d[0])
	if len(d) > 1 {
		b = append(b, '.')
		b = append(b, d[1:]...)
	}
	b = append(b, 'E')
	if adj >= 0 {
		b = append(b, '+')
	}
	return strconv.AppendInt(b, int64(adj), 10)
}

// round rounds t to n significant digits, ties to even.
func (t *text) round(n int) {
	d := t.digits()
	if n >= len(d) {
		return
	}
	if n < 0 {
		t.nd = 0
		return
	}
	up := d[n] > '5' || d[n] == '5' && (!allZero(d[n+1:]) || n > 0 && d[n-1]&1 != 0)
	t.nd = n
	if !up {
		return
	}
	for i := n - 1; i >= 0; i-- {
		if d[i] < '9' {
			d[i]++
			t.nd = i + 1
			return
		}
	}
	// All nines (or no digits at all): the value becomes 1 × 10**dp.
	t.off, t.nd = 0, 1
	t.buf[0] = '1'
	t.dp++
}

// trim drops the trailing zeros of the digits.
func (t *text) trim() {
	for t.nd > 0 && t.buf[t.off+t.nd-1] == '0' {
		t.nd--
	}
}

func allZero(d []byte) bool {
	for _, c := range d {
		if c != '0' {
			return false
		}
	}
	return true
}

// digit returns the digit with weight 10**(dp-1-i), which is zero outside
// the stored digits.
func (t *text) digit(i int) byte {
	if 0 <= i && i < t.nd {
		return t.buf[t.off+i]
	}
	return '0'
}

// append appends t in one of the strconv float formats 'e', 'E', 'f', 'F',
// 'g' or 'G'. A negative precision selects the exact representation.
func (t *text) append(b []byte, format byte, prec int) []byte {
	if t.neg {
		b = append(b, '-')
	}
	if b, ok := t.appendSpecial(b); ok {
		return b
	}
	exact := prec < 0
	switch format {
	case 'e', 'E':
		if exact {
			prec = max(t.nd-1, 0)
		}
		t.round(prec + 1)
		return t.appendE(b, format, prec)
	case 'f', 'F':
		if exact {
			prec = max(-t.exp, 0)
		}
		t.round(t.dp + prec)
		return t.appendF(b, prec)
	case 'g', 'G':
		if !exact {
			if prec == 0 {
				prec = 1
			}
			// As strconv: round to prec significant digits and drop the
			// trailing zeros, so that %.3g prints 1.00 as "1" and 1000 as
			// "1e+03".
			t.round(prec)
			t.trim()
		}
		if t.nd == 0 {
			t.dp = 0 // a zero of any exponent prints as "0"
		}
		eprec := prec
		if exact {
			eprec, prec = 21, t.nd
		} else if eprec > t.nd && t.nd >= t.dp {
			eprec = t.nd
		}
		// %e is used if the exponent is less than -4 or not less than the
		// precision, as in strconv.
		if x := t.dp - 1; x < -4 || x >= eprec {
			return t.appendE(b, format+'e'-'g', max(min(prec, t.nd)-1, 0))
		}
		if exact {
			return t.appendF(b, max(-t.exp, 0))
		}
		return t.appendF(b, max(t.nd-t.dp, 0))
	}
	return append(b, '%', format)
}

// appendE appends d.ddddde±dd.
func (t *text) appendE(b []byte, e byte, prec int) []byte {
	b = append(b, t.digit(0))
	if prec > 0 {
		b = append(b, '.')
		for i := 1; i <= prec; i++ {
			b = append(b, t.digit(i))
		}
	}
	x := t.dp - 1
	if t.nd == 0 {
		x = 0
	}
	b = append(b, e)
	if x < 0 {
		b, x = append(b, '-'), -x
	} else {
		b = append(b, '+')
	}
	if x < 10 {
		b = append(b, '0')
	}
	return strconv.AppendInt(b, int64(x), 10)
}

// appendF appends ddd.ddd with prec fractional digits.
func (t *text) appendF(b []byte, prec int) []byte {
	if t.nd == 0 || t.dp <= 0 {
		b = append(b, '0')
	} else {
		for i := 0; i < t.dp; i++ {
			b = append(b, t.digit(i))
		}
	}
	if prec > 0 {
		b = append(b, '.')
		for i := 0; i < prec; i++ {
			b = append(b, t.digit(t.dp+i))
		}
	}
	return b
}

// format implements fmt.Formatter for all the decimal types; typ names the
// type for the benefit of bad-verb reports.
func (t *text) format(s fmt.State, verb rune, typ string) {
	var scratch [64]byte
	b := scratch[:0]
	prec, hasPrec := s.Precision()
	if !hasPrec {
		prec = -1
	}
	neg := t.neg
	t.neg = false // the sign is laid out separately
	switch verb {
	case 'v', 's':
		b = t.appendSci(b)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		if !hasPrec && verb != 'g' && verb != 'G' {
			prec = 6
		}
		b = t.append(b, byte(verb), prec)
	default:
		t.neg = neg
		fmt.Fprintf(s, "%%!%c(decimal.%s=%s)", verb, typ, t.appendSci(nil))
		return
	}

	var sign string
	switch {
	case neg:
		sign = "-"
	case s.Flag('+') && verb != 'v':
		// fmt reports %+v as the '+' flag, but it asks for field names,
		// not a sign: float64 prints %+v of 1.5 as "1.5".
		sign = "+"
	case s.Flag(' '):
		sign = " "
	}
	width, _ := s.Width()
	pad := max(width-len(sign)-len(b), 0)
	switch {
	case s.Flag('-'):
		writeParts(s, sign, 0, b, pad)
	case s.Flag('0') && t.kind == finite:
		writeParts(s, sign, pad, b, 0)
	default:
		writePad(s, ' ', pad)
		writeParts(s, sign, 0, b, 0)
	}
}

func writeParts(s fmt.State, sign string, zeros int, b []byte, spaces int) {
	s.Write([]byte(sign))
	writePad(s, '0', zeros)
	s.Write(b)
	writePad(s, ' ', spaces)
}

func writePad(s fmt.State, c byte, n int) {
	var pad [16]byte
	for i := range pad {
		pad[i] = c
	}
	for ; n > 0; n -= len(pad) {
		s.Write(pad[:min(n, len(pad))])
	}
}
